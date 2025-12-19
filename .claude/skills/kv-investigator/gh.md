# GitHub CLI Commands for Issue/PR Discovery

Reference commands for searching and exploring issues/PRs in the CockroachDB repo.
Optimized for LLM consumption (JSON output, easy to filter with `jq`/`grep`).

## Discovery Search

Use when looking for potentially related issues (test failures, error messages, etc.).
The search input is typically an error message fragment or test name.

### Get result count first

Quick sanity check before fetching results:

```bash
gh api -X GET "search/issues?q=repo:cockroachdb/cockroach+<SEARCH_TERMS>&per_page=1" | jq '.total_count'
```

### Compact JSON listing (one object per line)

Best for piping to a file and exploring with `grep`/`jq`:

```bash
gh issue list --search "<SEARCH_TERMS> sort:updated-desc" --limit 30 --state all \
  --json number,title,state,labels,updatedAt \
  | jq -c '.[] | {n:.number, s:.state, u:(.updatedAt|split("T")[0]), t:.title, l:[.labels[].name]}'
```

Output:
```
{"n":157894,"s":"OPEN","u":"2025-12-18","t":"roachtest: multitenant-multiregion failed","l":["C-test-failure","O-robot","O-roachtest","branch-master","T-kv"]}
{"n":159050,"s":"CLOSED","u":"2025-12-17","t":"kv/kvserver/intentresolver: TestIntentResolutionUnavailableRange failed","l":["C-bug","C-test-failure","T-kv","P-3"]}
```

### Human-readable listing

For direct terminal output:

```bash
gh issue list --search "<SEARCH_TERMS> sort:updated-desc" --limit 30 --state all \
  --json number,title,state,labels,updatedAt,url \
  | jq -r '.[] | "[\(.state)] #\(.number) \(.title) (\(.updatedAt | split("T")[0]))\n  labels: \([.labels[].name] | join(", "))\n  \(.url)\n"'
```

### With body snippet

Useful for seeing error messages in results:

```bash
gh issue list --search "<SEARCH_TERMS> sort:updated-desc" --limit 15 --state all \
  --json number,title,state,labels,updatedAt,body \
  | jq -r '.[] | "[\(.state)] #\(.number) \(.title) (\(.updatedAt | split("T")[0]))\n  labels: \([.labels[].name] | join(", "))\n  snippet: \(if .body then (.body | gsub("\r"; "") | split("\n") | map(select(. != "")) | .[0:2] | join(" | ") | .[0:150]) else "" end)\n"'
```

## Deep-Dive View

Use when exploring a specific issue or PR in detail. Dump to file first, then explore.

### Issue (with comments and linked PRs)

GraphQL is needed to get `closedByPullRequestsReferences`:

```bash
gh api graphql -f query='
{
  repository(owner:"cockroachdb", name:"cockroach") {
    issue(number:ISSUE_NUMBER) {
      number
      title
      state
      body
      createdAt
      closedAt
      author { login }
      labels(first:20) { nodes { name } }
      comments(first:100) {
        nodes {
          author { login }
          createdAt
          body
        }
      }
      closedByPullRequestsReferences(first:10) {
        nodes {
          number
          title
          url
          state
          mergedAt
        }
      }
    }
  }
}' > /tmp/issue-ISSUE_NUMBER.json
```

### PR (with reviews, commits, files)

```bash
gh pr view PR_NUMBER \
  --json number,title,state,body,labels,comments,author,createdAt,updatedAt,url,reviews,commits,files,additions,deletions,mergedAt,mergedBy,reviewDecision,headRefName,baseRefName \
  > /tmp/pr-PR_NUMBER.json
```

## Search Syntax Tips

- `sort:updated-desc` - most recently updated first
- `sort:created-desc` - most recently created first  
- `in:title,body` - search in title and body (default)
- `label:C-test-failure` - filter by label
- `is:open` / `is:closed` - filter by state (or use `--state` flag)
- Quote exact phrases: `"context deadline exceeded"`
- OR queries: `TestFoo OR TestBar`

## Useful jq Filters

After dumping to a file, explore with:

```bash
# Pretty-print
jq . /tmp/issue-123.json

# Get just comments
jq '.data.repository.issue.comments.nodes[] | {author: .author.login, body: .body}' /tmp/issue-123.json

# Get linked PRs
jq '.data.repository.issue.closedByPullRequestsReferences.nodes' /tmp/issue-123.json

# Filter discovery results by label
cat /tmp/results.json | jq -c 'select(.l | index("T-kv"))'

# Filter by state
cat /tmp/results.json | jq -c 'select(.s == "CLOSED")'
```
