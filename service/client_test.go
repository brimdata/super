package service_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/api/client"
	"github.com/brimdata/super/compiler/srcfiles"
	"github.com/brimdata/super/db"
	dbapi "github.com/brimdata/super/db/api"
	"github.com/brimdata/super/db/branches"
	"github.com/brimdata/super/db/pools"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/sup"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
)

type testClient struct {
	*testing.T
	*client.Connection
}

func (c *testClient) TestPoolStats(id ksuid.KSUID) exec.PoolStats {
	r, err := c.Connection.PoolStats(c.Context(), id)
	require.NoError(c, err)
	return r
}

func (c *testClient) TestPoolGet(id ksuid.KSUID) (config pools.Config) {
	remote := dbapi.NewRemoteDB(c.Connection)
	pool, err := dbapi.LookupPoolByID(c.Context(), remote, id)
	require.NoError(c, err)
	return *pool
}

func (c *testClient) TestBranchGet(id ksuid.KSUID) (config db.BranchMeta) {
	remote := dbapi.NewRemoteDB(c.Connection)
	branch, err := dbapi.LookupBranchByID(c.Context(), remote, id)
	require.NoError(c, err)
	return *branch
}

func (c *testClient) TestPoolList() []pools.Config {
	r, err := c.Query(c.Context(), srcfiles.Plain("from :pools"))
	require.NoError(c, err)
	defer r.Body.Close()
	var confs []pools.Config
	zr := bsupio.NewReader(super.NewContext(), r.Body)
	defer zr.Close()
	for {
		rec, err := zr.Read()
		require.NoError(c, err)
		if rec == nil {
			return confs
		}
		var pool pools.Config
		err = sup.UnmarshalBSUP(*rec, &pool)
		require.NoError(c, err)
		confs = append(confs, pool)
	}
}

func (c *testClient) TestPoolPost(payload api.PoolPostRequest) ksuid.KSUID {
	r, err := c.Connection.CreatePool(c.Context(), payload)
	require.NoError(c, err)
	return r.Pool.ID
}

func (c *testClient) TestBranchPost(poolID ksuid.KSUID, payload api.BranchPostRequest) branches.Config {
	r, err := c.Connection.CreateBranch(c.Context(), poolID, payload)
	require.NoError(c, err)
	return r
}

func (c *testClient) TestQuery(query string) string {
	r, err := c.Connection.Query(c.Context(), srcfiles.Plain(query))
	require.NoError(c, err)
	defer r.Body.Close()
	zr := bsupio.NewReader(super.NewContext(), r.Body)
	defer zr.Close()
	var buf bytes.Buffer
	zw := supio.NewWriter(sio.NopCloser(&buf), supio.WriterOpts{})
	require.NoError(c, sio.Copy(zw, zr))
	return buf.String()
}

func (c *testClient) TestLoad(poolID ksuid.KSUID, branchName string, r io.Reader) ksuid.KSUID {
	commit, err := c.Connection.Load(c.Context(), poolID, branchName, "", r, api.CommitMessage{})
	require.NoError(c, err)
	return commit.Commit
}

func (c *testClient) TestAuthMethod() api.AuthMethodResponse {
	r, err := c.Connection.AuthMethod(c.Context())
	require.NoError(c, err)
	return r
}

func (c *testClient) TestAuthIdentity() api.AuthIdentityResponse {
	r, err := c.Connection.AuthIdentity(c.Context())
	require.NoError(c, err)
	return r
}
