package puller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/bruce34/grafana-dashboards-manager/internal/grafana"

	"github.com/bruce34/grafana-dashboards-manager/internal/config"

	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func getVersionsFile(prefix string) (filename string) {
	if prefix == "hostname" {
		hostname, _ := os.Hostname()
		return hostname + "-" + "versions-metadata.json"
	}
	return prefix + "versions-metadata.json"
}

// getDashboardsVersions reads the "versions.json" file at the root of the git
// repository and returns its content as a map.
// If the file doesn't exist, returns an empty map.
// Return an error if there was an issue looking for the file (except when the
// file doesn't exist), reading it or formatting its content into a map.
func GetDashboardsVersions(clonePath string, versionsFile string) (versions grafana.VersionFile, err error) {

	versions = grafana.VersionFile{
		DashboardMetaBySlug:    make(map[string]grafana.DbSearchResponse, 0),
		DashboardBySlug:        make(map[string]*grafana.Dashboard, 0),
		FoldersMetaByUID:       make(map[string]grafana.DbSearchResponse, 0),
		DashboardVersionBySlug: make(map[string]int, 0),
	}

	filename := clonePath + "/" + getVersionsFile(versionsFile)

	_, err = os.Stat(filename)
	if os.IsNotExist(err) {
		return versions, nil
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	if err = json.Unmarshal(data, &versions); err != nil {
		return
	}
	return
}

// writeVersions updates or creates the "versions.json" file at the root of the
// git repository. It takes as parameter a map of versions computed by
// getDashboardsVersions and a map linking a dashboard slug to an instance of
// diffVersion instance, and uses them both to compute an updated map of
// versions that it will convert to JSON, indent and write down into the
// "versions.json" file.
// Returns an error if there was an issue when conerting to JSON, indenting or
// writing on disk.
func writeVersions(versions grafana.VersionFile, dv map[string]diffVersion, clonePath string, versionsFile string,
) (err error) {
	rawJSON, err := json.Marshal(versions)
	if err != nil {
		return
	}

	indentedJSON, err := indent(rawJSON)
	if err != nil {
		return
	}

	filename := clonePath + "/" + getVersionsFile(versionsFile)
	return rewriteFile(filename, indentedJSON)
}

// commitNewVersions creates a git commit from updated dashboard files (that
// have previously been added to the git index) and an updated "versions.json"
// file that it creates (with writeVersions) and add to the index.
// Returns an error if there was an issue when creating the "versions.json"
// file, adding it to the index or creating the commit.
func commitNewVersions(versions grafana.VersionFile, dv map[string]diffVersion, worktree *gogit.Worktree,
	cfg *config.Config,
) (err error) {
	if err = writeVersions(versions, dv, cfg.Git.ClonePath, cfg.Git.VersionsFilePrefix); err != nil {
		return err
	}

	if _, err = worktree.Add(getVersionsFile(cfg.Git.VersionsFilePrefix)); err != nil {
		return err
	}
	_, err = worktree.Commit(getCommitMessage(dv), &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  cfg.Git.CommitsAuthor.Name,
			Email: cfg.Git.CommitsAuthor.Email,
			When:  time.Now(),
		},
	})

	return
}

// getCommitMessage creates a commit message that summarises the version updates
// included in the commit.
func getCommitMessage(dv map[string]diffVersion) string {
	hostname, _ := os.Hostname()

	message := "Updated dashboards on " + hostname + "\n"

	for slug, diff := range dv {
		message += fmt.Sprintf(
			"%s: %d => %d\n", slug, diff.old, diff.new,
		)
	}

	return message
}
