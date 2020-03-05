package zqd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/brimsec/zq/pcap"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zio/detector"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zqd/api"
	"github.com/brimsec/zq/zqd/packet"
	"github.com/brimsec/zq/zqd/search"
	"github.com/brimsec/zq/zqd/space"
	"github.com/gorilla/mux"
)

var taskCount int64

func handleSearch(root string, w http.ResponseWriter, r *http.Request) {
	var req api.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s, err := space.Open(root, req.Space)
	if err != nil {
		status := http.StatusInternalServerError
		if err == space.ErrSpaceNotExist {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	var out search.Output
	format := r.URL.Query().Get("format")
	switch format {
	case "zjson", "json":
		// XXX Should write appropriate ndjson content header.
		out = search.NewJSONOutput(w, search.DefaultMTU)
	case "bzng":
		// XXX Should write appropriate bzng content header.
		out = search.NewBzngOutput(w)
	default:
		http.Error(w, fmt.Sprintf("unsupported output format: %s", format), http.StatusBadRequest)
	}
	// XXX This always returns bad request but should return status codes
	// that reflect the nature of the returned error.
	if err := search.Search(r.Context(), s, req, out); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func handlePacketSearch(root string, w http.ResponseWriter, r *http.Request) {
	s := extractSpace(root, w, r)
	if s == nil {
		return
	}
	req := &api.PacketSearch{}
	if err := req.FromQuery(r.URL.Query()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	if s.PacketPath() == "" || !s.HasFile(packet.IndexFile) {
		http.Error(w, "space has no pcaps", http.StatusNotFound)
		return
	}
	index, err := pcap.LoadIndex(s.DataPath(packet.IndexFile))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f, err := os.Open(s.PacketPath())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	slicer, err := pcap.NewSlicer(f, index, req.Span)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var search *pcap.Search
	switch req.Proto {
	default:
		msg := fmt.Sprintf("unsupported proto type: %s", req.Proto)
		http.Error(w, msg, http.StatusBadRequest)
		return
	case "tcp":
		flow := pcap.NewFlow(req.SrcHost, int(req.SrcPort), req.DstHost, int(req.DstPort))
		search = pcap.NewTCPSearch(req.Span, flow)
	case "udp":
		flow := pcap.NewFlow(req.SrcHost, int(req.SrcPort), req.DstHost, int(req.DstPort))
		search = pcap.NewUDPSearch(req.Span, flow)
	case "icmp":
		search = pcap.NewICMPSearch(req.Span, req.SrcHost, req.DstHost)
	}
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s.pcap", search.ID()))
	err = search.Run(w, slicer)
	if err != nil {
		if err == pcap.ErrNoPacketsFound {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
}

func handleSpaceList(root string, w http.ResponseWriter, r *http.Request) {
	info, err := ioutil.ReadDir(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	spaces := []string{}
	for _, subdir := range info {
		if !subdir.IsDir() {
			continue
		}
		s, err := space.Open(root, subdir.Name())
		if err != nil {
			continue
		}
		spaces = append(spaces, s.Name())
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spaces); err != nil {
		// XXX Add zap here.
		log.Println("Error writing response", err)
	}
}

func handleSpaceGet(root string, w http.ResponseWriter, r *http.Request) {
	s := extractSpace(root, w, r)
	if s == nil {
		return
	}
	f, err := s.OpenFile("all.bzng")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer f.Close()
	// XXX This is slow. Can easily cache result rather than scanning
	// whole file each time.
	reader, err := detector.LookupReader("bzng", f, resolver.NewContext())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	minTs := nano.MaxTs
	maxTs := nano.MinTs
	var found bool
	for {
		rec, err := reader.Read()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if rec == nil {
			break
		}
		ts := rec.Ts
		if ts < minTs {
			minTs = ts
		}
		if ts > maxTs {
			maxTs = ts
		}
		found = true
	}
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	info := &api.SpaceInfo{
		Name:          s.Name(),
		Size:          stat.Size(),
		PacketSupport: s.HasFile(packet.IndexFile),
		PacketPath:    s.PacketPath(),
	}
	if found {
		info.MinTime = &minTs
		info.MaxTime = &maxTs
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func handleSpacePost(root string, w http.ResponseWriter, r *http.Request) {
	var req api.SpacePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s, err := space.Create(root, req.Name, req.DataDir)
	if err != nil {
		status := http.StatusInternalServerError
		if err == space.ErrSpaceExists {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	res := api.SpacePostResponse{
		Name:    s.Name(),
		DataDir: s.DataPath(),
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		// XXX Add zap here.
		log.Println("Error writing response", err)
	}
}

func handleSpaceDelete(root string, w http.ResponseWriter, r *http.Request) {
	s := extractSpace(root, w, r)
	if s == nil {
		return
	}
	if err := s.Delete(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handlePacketPost(root string, w http.ResponseWriter, r *http.Request) {
	s := extractSpace(root, w, r)
	if s == nil {
		return
	}
	var req api.PacketPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	proc, err := packet.IngestFile(r.Context(), s, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusCreated)
	pipe := api.NewJSONPipe(w)
	taskId := atomic.AddInt64(&taskCount, 1)
	taskStart := api.TaskStart{Type: "TaskStart", TaskID: taskId}
	if err = pipe.Send(taskStart); err != nil {
		// Probably an error writing to socket, log error.
		// XXX This should be zap instead.
		log.Print(err)
		return
	}
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	for {
		var done bool
		select {
		case <-proc.Done():
			done = true
		case <-ticker.C:
		}
		status := api.PacketPostStatus{
			Type:           "PacketPostStatus",
			StartTime:      proc.StartTime,
			UpdateTime:     nano.Now(),
			PacketSize:     proc.PcapSize,
			PacketReadSize: proc.PcapReadSize(),
		}
		if err := pipe.Send(status); err != nil {
			// XXX This should be zap instead.
			log.Print(err)
			return
		}
		if done {
			break
		}
	}
	taskEnd := api.TaskEnd{Type: "TaskEnd", TaskID: taskId}
	if err != nil {
		var ok bool
		taskEnd.Error, ok = err.(*api.Error)
		if !ok {
			taskEnd.Error = &api.Error{Type: "Error", Message: err.Error()}
		}
	}
	if err = pipe.SendFinal(taskEnd); err != nil {
		// XXX This should be zap instead.
		log.Print(err)
		return
	}
}

func extractSpace(root string, w http.ResponseWriter, r *http.Request) *space.Space {
	name := extractSpaceName(w, r)
	if name == "" {
		return nil
	}
	s, err := space.Open(root, name)
	if err != nil {
		if err == space.ErrSpaceNotExist {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return nil
	}
	return s
}

// extractSpaceName returns the unescaped space from the path of a request.
func extractSpaceName(w http.ResponseWriter, r *http.Request) string {
	v := mux.Vars(r)
	space, ok := v["space"]
	if !ok {
		http.Error(w, "no space name in path", http.StatusBadRequest)
		return ""
	}
	return space
}
