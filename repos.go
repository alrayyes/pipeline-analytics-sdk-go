package pipelineanalytics

import (
	"context"
	"fmt"
	"iter"

	"github.com/alrayyes/pipeline-analytics-sdk-go/internal/genclient"
)

// defaultPageSize is used when the caller's params leave Limit unset --
// ListRepos never paginates on its own (RepoList.HasMore is always false
// with no limit), so an iterator needs one to actually walk pages.
const defaultPageSize = 50

// ListRepos returns an iterator over every repo matching params, walking
// offset pages transparently. Range over it with a for-range loop; a
// non-nil error from Err (checked once the loop ends) means the walk
// stopped early on either a transport failure or an API error.
func (c *Client) ListRepos(ctx context.Context, params *genclient.ListReposParams) *RepoIterator {
	p := genclient.ListReposParams{}
	if params != nil {
		p = *params
	}
	if p.Limit == nil {
		limit := genclient.Limit(defaultPageSize)
		p.Limit = &limit
	}

	return &RepoIterator{ctx: ctx, client: c, params: p}
}

// RepoIterator walks every page of a ListRepos call. Get Err after
// ranging to see whether the walk completed or stopped on an error.
type RepoIterator struct {
	ctx    context.Context
	client *Client
	params genclient.ListReposParams
	err    error
}

// Err returns the error that stopped iteration, or nil if it ran to
// completion (or hasn't started).
func (it *RepoIterator) Err() error { return it.err }

// All returns a range-over-func iterator (Go 1.23+) yielding one Repo per
// call: `for repo := range it.All() { ... }`.
func (it *RepoIterator) All() iter.Seq[genclient.Repo] {
	return func(yield func(genclient.Repo) bool) {
		offset := it.startOffset()

		for {
			page, hasMore, err := it.fetchPage(offset)
			if err != nil {
				it.err = err

				return
			}

			if !yieldAll(page, yield) {
				return
			}

			if !hasMore {
				return
			}

			offset += genclient.Offset(len(page))
		}
	}
}

func (it *RepoIterator) startOffset() genclient.Offset {
	if it.params.Offset != nil {
		return *it.params.Offset
	}

	return 0
}

// yieldAll calls yield for every repo in page, stopping early (and
// returning false) the moment yield does.
func yieldAll(page []genclient.Repo, yield func(genclient.Repo) bool) bool {
	for _, repo := range page {
		if !yield(repo) {
			return false
		}
	}

	return true
}

// fetchPage fetches one page of repos starting at offset.
func (it *RepoIterator) fetchPage(offset genclient.Offset) ([]genclient.Repo, bool, error) {
	params := it.params
	params.Offset = &offset

	resp, err := it.client.ListReposWithResponse(it.ctx, &params)
	if err != nil {
		return nil, false, fmt.Errorf("pipeline-analytics: list repos: %w", err)
	}

	if apiErr := DecodeError(resp.HTTPResponse.StatusCode, resp.Body, resp.HTTPResponse.Header); apiErr != nil {
		return nil, false, apiErr
	}

	return resp.JSON200.Repos, resp.JSON200.HasMore, nil
}
