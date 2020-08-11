package space

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strings"
	"unicode"

	"github.com/brimsec/zq/pkg/iosrc"
	"github.com/brimsec/zq/zqd/api"
	"github.com/brimsec/zq/zqd/storage"
	"github.com/brimsec/zq/zqe"
	"github.com/segmentio/ksuid"
)

func invalidSpaceNameRune(r rune) bool {
	return r == '/' || !unicode.IsPrint(r)
}

func validSpaceName(s string) bool {
	return strings.IndexFunc(s, invalidSpaceNameRune) == -1
}

const configVersion = 2

type config struct {
	Version   int              `json:"version"`
	Name      string           `json:"name"`
	DataURI   iosrc.URI        `json:"data_uri"`
	PcapPath  string           `json:"pcap_path"`
	Storage   storage.Config   `json:"storage"`
	Subspaces []subspaceConfig `json:"subspaces"`
}

type configV1 struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	DataPath string `json:"data_path"`
	// XXX PcapPath should be named pcap_path in json land. To avoid having to
	// do a migration we'll keep this as-is for now.
	PcapPath  string           `json:"packet_path"`
	Storage   storage.Config   `json:"storage"`
	Subspaces []subspaceConfig `json:"subspaces"`
}

// versionCheck is used to establish the version of the loaded config file.
// This must always remain the same as the Version field in config.
type versionCheck struct {
	Version int `json:"version"`
}

type subspaceConfig struct {
	ID          api.SpaceID                `json:"id"`
	Name        string                     `json:"name"`
	OpenOptions storage.ArchiveOpenOptions `json:"open_options"`
}

func (c config) clone() config {
	n := c
	n.Subspaces = append([]subspaceConfig{}, c.Subspaces...)
	return n
}

func (c config) subspaceIndex(id api.SpaceID) int {
	for i, sub := range c.Subspaces {
		if sub.ID == id {
			return i
		}
	}
	return -1
}

// loadConfig loads the contents of config.json in a space's path.
func loadConfig(spaceURI iosrc.URI) (config, error) {
	var c config
	p := spaceURI.AppendPath(configFile)
	r, err := iosrc.NewReader(p)
	if err != nil {
		return c, err
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return c, err
	}
	if err := r.Close(); err != nil {
		return c, err
	}
	var vc versionCheck
	if err := json.Unmarshal(data, &vc); err != nil {
		return c, err
	}
	if vc.Version > configVersion {
		return c, fmt.Errorf("space config version %d ahead of binary version %d", vc.Version, configVersion)
	}
	if vc.Version < configVersion {
		return migrateConfig(vc.Version, data, spaceURI)
	}
	return c, json.Unmarshal(data, &c)
}

type migrator func([]byte, iosrc.URI) (int, []byte, error)

func migrateConfig(version int, data []byte, spaceURI iosrc.URI) (config, error) {
	var m migrator
	for version < configVersion {
		switch version {
		case 0:
			m = migrateConfigV1
		case 1:
			m = migrateConfigV2
		default:
			return config{}, fmt.Errorf("unsupported config migration %d", version)
		}
		var err error
		if version, data, err = m(data, spaceURI); err != nil {
			return config{}, err
		}
	}
	var c config
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, writeConfig(spaceURI, c)
}

func migrateConfigV1(data []byte, spaceURI iosrc.URI) (int, []byte, error) {
	var c configV1
	if err := json.Unmarshal(data, &c); err != nil {
		return 0, nil, err
	}
	if c.Name == "" {
		// Ensure that name is not blank for spaces created before the
		// zq#721 work to use space ids.
		c.Name = path.Base(spaceURI.Path)
	} else {
		// In version 0 names were not required to be unique. Since this should
		// be a rare case that any version 0 config files exist, append a uuid
		// to ensure uniqueness.
		id := ksuid.New()
		c.Name = c.Name + "_" + id.String()
		c.Name = safeName(c.Name)
	}
	if c.Storage.Kind == storage.UnknownStore {
		c.Storage.Kind = storage.FileStore
	}
	d, err := json.Marshal(c)
	return 1, d, err
}

func migrateConfigV2(data []byte, _ iosrc.URI) (int, []byte, error) {
	var v1 configV1
	if err := json.Unmarshal(data, &v1); err != nil {
		return 0, nil, err
	}
	if v1.DataPath == "." {
		v1.DataPath = ""
	}
	du, err := iosrc.ParseURI(v1.DataPath)
	if err != nil {
		return 0, nil, err
	}
	c := config{
		Version:   2,
		Name:      v1.Name,
		DataURI:   du,
		PcapPath:  v1.PcapPath,
		Storage:   v1.Storage,
		Subspaces: v1.Subspaces,
	}
	d, err := json.Marshal(c)
	return 2, d, err
}

func writeConfig(spaceURI iosrc.URI, c config) error {
	src, err := iosrc.GetSource(spaceURI)
	if err != nil {
		return err
	}
	uri := spaceURI.AppendPath(configFile)
	var w io.WriteCloser
	if replacer, ok := src.(iosrc.ReplacerAble); ok {
		w, err = replacer.NewReplacer(uri)
	} else {
		w, err = src.NewWriter(uri)
	}
	if err != nil {
		return err
	}
	if err := json.NewEncoder(w).Encode(c); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func validateName(names map[string]api.SpaceID, name string) error {
	if name == "" {
		return zqe.E(zqe.Invalid, "cannot set name to an empty string")
	}
	if !validSpaceName(name) {
		return zqe.E(zqe.Invalid, "name may not contain '/' or non-printable characters")
	}
	if _, ok := names[name]; ok {
		return zqe.E(zqe.Conflict, "space with name '%s' already exists", name)
	}
	return nil
}

// safeName converts the proposed name to a name that adheres to the constraints
// placed on a space's name (i.e. follows the name regex).
func safeName(proposed string) string {
	var sb strings.Builder
	for _, r := range proposed {
		if invalidSpaceNameRune(r) {
			r = '_'
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func uniqueName(names map[string]api.SpaceID, proposed string) string {
	name := proposed
	for i := 1; ; i++ {
		if _, ok := names[name]; !ok {
			return name
		}
		name = fmt.Sprintf("%s_%02d", proposed, i)
	}
}
