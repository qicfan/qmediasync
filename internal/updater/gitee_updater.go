package updater

import (
	"Q115-STRM/internal/helpers"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
)

type GiteeRelease struct {
	ID          int64        `json:"id"`
	TagName     string       `json:"tag_name"`
	Name        string       `json:"name"`
	Body        string       `json:"body"`
	Draft       bool         `json:"draft"`
	Prerelease  bool         `json:"prerelease"`
	PublishedAt string       `json:"created_at"`
	Assets      []GiteeAsset `json:"assets"`
	HTMLURL     string       `json:"html_url"`
}

type GiteeAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

type GiteeUpdater struct {
	Owner             string
	Repo              string
	CurrentVersion    string
	HTTPClient        *http.Client
	IncludePreRelease bool
}

func NewGiteeUpdater(owner, repo, currentVersion string) *GiteeUpdater {
	return &GiteeUpdater{
		Owner:          owner,
		Repo:           repo,
		CurrentVersion: currentVersion,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (g *GiteeUpdater) getReleases() ([]GiteeRelease, error) {
	apiUrl := fmt.Sprintf("https://gitee.com/api/v5/repos/%s/%s/releases", g.Owner, g.Repo)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", g.Repo+"-updater")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gitee API returned status: %d", resp.StatusCode)
	}

	var releases []GiteeRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	return releases, nil
}

func (g *GiteeUpdater) findMatchingAsset(assets []GiteeAsset) (string, string) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goarch == "amd64" {
		goarch = "x86_64"
	}
	filename := fmt.Sprintf("QMediaSync_%s_%s", goos, goarch)
	if goos == "windows" {
		filename += ".zip"
	} else {
		filename += ".tar.gz"
	}
	helpers.AppLogger.Infof("[Gitee] 查找资源文件: %s", filename)
	var downloadURL string

	for _, asset := range assets {
		name := asset.Name
		if strings.EqualFold(name, filename) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return downloadURL, ""
}

func (g *GiteeUpdater) CheckForUpdate() (*UpdateInfo, error) {
	releases, err := g.GetLatestReleases(1)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	latestRelease := releases[0]

	currentVer, err := version.NewVersion(g.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	latestVer, err := version.NewVersion(latestRelease.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid latest version: %w", err)
	}

	hasUpdate := latestVer.GreaterThan(currentVer)

	fullReleases, err := g.getReleases()
	if err != nil {
		return nil, err
	}

	var downloadURL, checksum string
	for _, release := range fullReleases {
		if release.TagName == latestRelease.Version {
			downloadURL, checksum = g.findMatchingAsset(release.Assets)
			break
		}
	}

	return &UpdateInfo{
		LatestVersion:  latestRelease.Version,
		CurrentVersion: g.CurrentVersion,
		ReleaseNotes:   latestRelease.ReleaseNotes,
		DownloadURL:    downloadURL,
		Checksum:       checksum,
		PublishedAt:    latestRelease.PublishedAt,
		HasUpdate:      hasUpdate,
	}, nil
}

func (g *GiteeUpdater) GetLatestReleases(limit int) ([]ReleaseInfo, error) {
	releases, err := g.getReleases()
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}

	filteredReleases := g.filterAndSortReleases(releases, limit)

	result := make([]ReleaseInfo, 0, len(filteredReleases))
	for _, release := range filteredReleases {
		publishedAt, _ := time.Parse(time.RFC3339, release.PublishedAt)

		result = append(result, ReleaseInfo{
			Version:      release.TagName,
			Name:         release.Name,
			ReleaseNotes: release.Body,
			PublishedAt:  publishedAt,
			IsPrerelease: release.Prerelease,
			IsDraft:      release.Draft,
			PageURL:      release.HTMLURL,
		})
	}

	return result, nil
}

func (g *GiteeUpdater) GetLatestStableReleases(limit int) ([]ReleaseInfo, error) {
	originalSetting := g.IncludePreRelease
	g.IncludePreRelease = false
	defer func() { g.IncludePreRelease = originalSetting }()

	return g.GetLatestReleases(limit)
}

func (g *GiteeUpdater) filterAndSortReleases(releases []GiteeRelease, limit int) []GiteeRelease {
	filtered := make([]GiteeRelease, 0, len(releases))

	for _, release := range releases {
		if release.Draft {
			continue
		}
		if !g.IncludePreRelease && release.Prerelease {
			continue
		}
		filtered = append(filtered, release)
	}

	sort.Slice(filtered, func(i, j int) bool {
		timeI, _ := time.Parse(time.RFC3339, filtered[i].PublishedAt)
		timeJ, _ := time.Parse(time.RFC3339, filtered[j].PublishedAt)
		return timeI.After(timeJ)
	})

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered
}

func (g *GiteeUpdater) GetReleaseDownloadURL(versionTag string) (string, string, *ReleaseInfo, error) {
	releases, err := g.getReleases()
	if err != nil {
		return "", "", nil, fmt.Errorf("获取releases失败: %w", err)
	}
	helpers.AppLogger.Infof("[Gitee] 获取到 %d 个 releases", len(releases))

	var targetRelease *GiteeRelease
	for i := range releases {
		helpers.AppLogger.Infof("[Gitee] 检查版本: %s", releases[i].TagName)
		if releases[i].TagName == versionTag {
			targetRelease = &releases[i]
			break
		}
	}

	if targetRelease == nil {
		return "", "", nil, fmt.Errorf("未找到版本: %s", versionTag)
	}

	downloadURL, checksumURL := g.findMatchingAsset(targetRelease.Assets)

	if downloadURL == "" {
		return "", "", nil, fmt.Errorf("未找到适合当前系统的下载文件")
	}

	publishedAt, _ := time.Parse(time.RFC3339, targetRelease.PublishedAt)

	releaseInfo := &ReleaseInfo{
		Version:      targetRelease.TagName,
		Name:         targetRelease.Name,
		ReleaseNotes: targetRelease.Body,
		PublishedAt:  publishedAt,
		IsPrerelease: targetRelease.Prerelease,
		IsDraft:      targetRelease.Draft,
		PageURL:      targetRelease.HTMLURL,
	}

	return downloadURL, checksumURL, releaseInfo, nil
}
