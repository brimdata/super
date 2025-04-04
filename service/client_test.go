package service_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/api/client"
	"github.com/brimdata/super/lake"
	lakeapi "github.com/brimdata/super/lake/api"
	"github.com/brimdata/super/lake/branches"
	"github.com/brimdata/super/lake/pools"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/bsupio"
	"github.com/brimdata/super/zio/supio"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
)

type testClient struct {
	*testing.T
	*client.Connection
}

func (c *testClient) TestPoolStats(id ksuid.KSUID) exec.PoolStats {
	r, err := c.Connection.PoolStats(context.Background(), id)
	require.NoError(c, err)
	return r
}

func (c *testClient) TestPoolGet(id ksuid.KSUID) (config pools.Config) {
	remote := lakeapi.NewRemoteLake(c.Connection)
	pool, err := lakeapi.LookupPoolByID(context.Background(), remote, id)
	require.NoError(c, err)
	return *pool
}

func (c *testClient) TestBranchGet(id ksuid.KSUID) (config lake.BranchMeta) {
	remote := lakeapi.NewRemoteLake(c.Connection)
	branch, err := lakeapi.LookupBranchByID(context.Background(), remote, id)
	require.NoError(c, err)
	return *branch
}

func (c *testClient) TestPoolList() []pools.Config {
	r, err := c.Query(context.Background(), "from :pools")
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
	r, err := c.Connection.CreatePool(context.Background(), payload)
	require.NoError(c, err)
	return r.Pool.ID
}

func (c *testClient) TestBranchPost(poolID ksuid.KSUID, payload api.BranchPostRequest) branches.Config {
	r, err := c.Connection.CreateBranch(context.Background(), poolID, payload)
	require.NoError(c, err)
	return r
}

func (c *testClient) TestQuery(query string) string {
	r, err := c.Connection.Query(context.Background(), query)
	require.NoError(c, err)
	defer r.Body.Close()
	zr := bsupio.NewReader(super.NewContext(), r.Body)
	defer zr.Close()
	var buf bytes.Buffer
	zw := supio.NewWriter(zio.NopCloser(&buf), supio.WriterOpts{})
	require.NoError(c, zio.Copy(zw, zr))
	return buf.String()
}

func (c *testClient) TestLoad(poolID ksuid.KSUID, branchName string, r io.Reader) ksuid.KSUID {
	commit, err := c.Connection.Load(context.Background(), poolID, branchName, "", r, api.CommitMessage{})
	require.NoError(c, err)
	return commit.Commit
}

func (c *testClient) TestAuthMethod() api.AuthMethodResponse {
	r, err := c.Connection.AuthMethod(context.Background())
	require.NoError(c, err)
	return r
}

func (c *testClient) TestAuthIdentity() api.AuthIdentityResponse {
	r, err := c.Connection.AuthIdentity(context.Background())
	require.NoError(c, err)
	return r
}
