package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mfojtik/ocp-what-merged/pkg/release"

	"github.com/openshift/library-go/pkg/image/reference"

	"github.com/dustin/go-humanize"
	"github.com/google/go-github/github"
	"github.com/lensesio/tableprinter"
	"github.com/xhit/go-str2duration/v2"
	"github.com/xxjwxc/gowp/workpool"
	"golang.org/x/oauth2"
)

type RepositoryCommit struct {
	// URL is commit URL
	URL string `header:"URL"`
	// Message is commit message
	Message string `header:"Message"`
	// RelativeTime is a string with relative time of the commit to current time
	RelativeTime string `header:"When"`

	// committed is needed to sort the commits by time
	committed time.Time
}

type commitListOptions struct {
	concurrency int
	since       time.Duration
	until       time.Duration
	branchName  string
}

// splitRepositoryURL splits the github repository URL to org ane repo name.
// this is needed because GH client does not fill these values in Commit object for some reason
func splitRepositoryURL(repository string) (string, string, bool) {
	if !strings.HasPrefix(repository, "https://github.com/") {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(repository, "https://github.com/"), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// listRepositoryCommits lists the commits in single repository
func listRepositoryCommits(ctx context.Context, client *github.Client, repository string, options commitListOptions) ([]*github.RepositoryCommit, error) {
	organization, name, ok := splitRepositoryURL(repository)
	if !ok {
		return nil, fmt.Errorf("unable to parse repository organization or name: %q", repository)
	}

	since := time.Now().Add(-options.since)
	until := time.Now()
	if options.until > 0 {
		until = since.Add(options.until)
	}

	commits, _, err := client.Repositories.ListCommits(ctx, organization, name, &github.CommitsListOptions{
		SHA:   options.branchName,
		Since: since,
		Until: until,
	})

	// TODO: skip repositories that does not exists in this payload... probably wrong combination of payload (--release) and branch name.
	// TODO: this should probably throw some warning.
	if err != nil && strings.Contains(err.Error(), "404 Not Found") {
		return []*github.RepositoryCommit{}, nil
	}
	return commits, err
}

// isMergeCommit tell us if commit is a merge commit, because we don't want to print those
// this is weak, but cheap and does not require extra request to GH API
func isMergeCommit(commit *github.Commit) bool {
	return strings.Contains(commit.GetMessage(), "Merge pull request")
}

// cleanupCommitMessage make the message suitable for printing into console.
// it removes the signed-off thing, trim it to 80 chars max and remove extra spaces.
func cleanupCommitMessage(msg string) string {
	lines := strings.Split(msg, "\n")
	var r []string
	for _, l := range lines {
		// filter out signatures from commit messages
		if strings.Contains(l, "Signed-off-by") || len(strings.TrimSpace(l)) == 0 {
			continue
		}
		// trim the length of each line to 80 characters
		if len(l) > 80 {
			l = l[0:80] + " ..."
		}
		r = append(r, strings.TrimSpace(l))
	}
	return strings.Join(r, "\n")
}

// cleanupCommitURL makes the commit URL shorter, because we don't need a full SHA which fills the terminal
func cleanupCommitURL(u string) string {
	parts := strings.Split(u, "/")
	parts[len(parts)-1] = parts[len(parts)-1][:6]
	return strings.Join(parts, "/")
}

// listCommitsForRepositories will take list of repositories and return the changes
func listCommitsForRepositories(ctx context.Context, client *github.Client, options commitListOptions, repositories []string) ([]RepositoryCommit, error) {
	wp := workpool.New(options.concurrency)
	var changes []RepositoryCommit
	var commitsLock sync.Mutex
	var tasks []workpool.TaskHandler

	for i := range repositories {
		repository := &repositories[i]
		tasks = append(tasks, func() error {
			result, err := listRepositoryCommits(ctx, client, *repository, options)
			if err != nil {
				return err
			}
			var change []RepositoryCommit
			for _, c := range result {
				// skip merge commits
				if isMergeCommit(c.GetCommit()) {
					continue
				}
				change = append(change, RepositoryCommit{
					URL:          cleanupCommitURL(c.GetHTMLURL()),
					Message:      cleanupCommitMessage(c.GetCommit().GetMessage()),
					RelativeTime: humanize.Time(c.GetCommit().GetCommitter().GetDate()),
					committed:    c.GetCommit().GetCommitter().GetDate(),
				})
			}

			commitsLock.Lock()
			defer commitsLock.Unlock()
			changes = append(changes, change...)
			return nil
		})
	}

	// schedule all tasks, the work pool will take care of queuing
	for i := range tasks {
		wp.Do(tasks[i])
	}
	if err := wp.Wait(); err != nil {
		return nil, err
	}

	// sort by time, from oldest to latest
	sort.Slice(changes, func(i, j int) bool {
		return changes[j].committed.After(changes[i].committed)
	})
	return changes, nil
}

func main() {
	var (
		since      string
		until      string
		branch     string
		releaseTag string
		tags       bool
	)

	flag.StringVar(&branch, "branch", "master", "Branch name to use for search (eg. 'release-4.6', ...)")
	flag.StringVar(&since, "since", "1d", "Relative time to search the commits from (eg. '1d', '48h', ...)")
	flag.StringVar(&until, "until", "", "Relative time to since. If since is 5d and until is 1d, it will show commits between 5d ago to 4d ago")
	flag.StringVar(&releaseTag, "tag", "", "Release tag to use instead of latest (--tags will list all available tags)")
	flag.BoolVar(&tags, "tags", false, "List all available release tags (use -release 4.6 to filter only tags for 4.6 release)")

	flag.Parse()

	githubToken := os.Getenv("GITHUB_TOKEN")
	if len(githubToken) == 0 {
		log.Fatal(":-( I need you to set GITHUB_TOKEN env variable in order to be able to talk to Github")
	}
	client := github.NewClient(oauth2.NewClient(context.TODO(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})))

	commitListOptions := commitListOptions{
		concurrency: 10,
	}

	if len(since) > 0 {
		var err error
		commitListOptions.since, err = str2duration.ParseDuration(since)
		if err != nil {
			log.Fatalf(":-( I am unable to parse duration %q (must be like 1h or 1d...)", since)
		}
	}
	if len(until) > 0 {
		var err error
		commitListOptions.until, err = str2duration.ParseDuration(until)
		if err != nil {
			log.Fatalf(":-( I am unable to parse duration %q (must be like 1h or 1d...)", until)
		}
	}
	if len(branch) > 0 {
		commitListOptions.branchName = branch
	}

	ctx := context.Background()

	// initialize the docker client
	// this make connection to quay.io and check if the release repository is available
	reposOptions, err := release.New(ctx)
	if err != nil {
		log.Fatalf(":-( unable to connect to OCP release registry: %v", err)
	}

	// if -tags is used, do nothing just list the tags
	// if -tag is used, then filter the list to include only tags starting with the value specified
	if tags {
		filterFuncs := []func(string) bool{
			release.FilterByArch("x86_64"),
		}
		if len(releaseTag) > 0 {
			filterFuncs = append(filterFuncs, release.FilterByPrefix(releaseTag))
		}
		tags, err := reposOptions.ListReleaseTags(ctx, filterFuncs...)
		if err != nil {
			log.Fatalf("unable to list release tags: %v", err)
		}
		fmt.Fprintf(os.Stdout, "%s\n", strings.Join(tags, ", "))
		os.Exit(0)
	}

	// now get the list of repositories in payload determined by tag we use
	var targetRelease *reference.DockerImageReference
	if len(releaseTag) > 0 {
		targetRelease, err = reposOptions.GetReleaseTagReference(ctx, releaseTag)
	} else {
		targetRelease, err = reposOptions.GetLatestReleaseTag(ctx)
	}
	if err != nil {
		log.Fatalf("unable to get release payload with tag %q: %v", releaseTag, err)
	}
	repos, err := reposOptions.GetPayloadRepositories(ctx, *targetRelease)
	if err != nil {
		log.Fatal(err)
	}

	// we have repositories, now ask github to list commits for all of them
	changes, err := listCommitsForRepositories(ctx, client, commitListOptions, repos)
	if err != nil {
		log.Fatal(err)
	}

	printer := tableprinter.New(os.Stdout)
	printer.Print(changes)

	fmt.Printf("\n%d repositories processed for commits to %s branch (payload:%s), since %s\n", len(repos), commitListOptions.branchName, targetRelease.Registry+"/"+targetRelease.Namespace+"/"+targetRelease.Name+":"+targetRelease.Tag, commitListOptions.since)
}
