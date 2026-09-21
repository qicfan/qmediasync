package syncstrm

import (
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Issue #258 的场景：本地→网盘同步（网盘是挂载到本地的目录，如 CloudDrive2 挂载 115），
// 网盘上已存在 电视剧/唐朝诡事录 (2022)/Season.04，本地刮削新增 tvshow.nfo 需要上传。
// 目录名里带空格和括号，用于覆盖路径解析中可能出现问题的特殊字符
const (
	testDramaName = "唐朝诡事录 (2022)"
	testSeasonDir = "Season.04"
)

// newTestSyncStrm 构造一个只用于路径计算的 SyncStrm（不需要数据库和日志）
func newTestSyncStrm(targetPath, sourcePath, sourcePathId string) *SyncStrm {
	s := &SyncStrm{TargetPath: targetPath, SourcePath: sourcePath, SourcePathId: sourcePathId}
	s.Sync = &models.Sync{LocalPath: targetPath, RemotePath: sourcePath, BaseCid: sourcePathId}
	return s
}

// newLocalDirCacheEntry 模拟同步遍历网盘（挂载目录）时写入内存缓存的目录记录：
// 本地类型的 SyncFileCache.Path 恒为空，真实网盘路径保存在 ParentId 中
func newLocalDirCacheEntry(remoteParentDir, dirName string) *SyncFileCache {
	return &SyncFileCache{
		ParentId:   remoteParentDir,
		FileName:   dirName,
		FileType:   v115open.TypeDir,
		SourceType: models.SourceTypeLocal,
	}
}

// 网盘上已存在该父目录时，上传路径必须保留全部中间层级（Issue #258 的根因）
func TestUploadRemoteDirPathLocalParentDirExists(t *testing.T) {
	base := t.TempDir()
	remoteRoot := filepath.ToSlash(filepath.Join(base, "CloudDrive2", "115", "媒体库", "电视剧"))
	localRoot := filepath.ToSlash(filepath.Join(base, "strm", "电视剧"))
	s := newTestSyncStrm(localRoot, remoteRoot, remoteRoot)

	existsPath := newLocalDirCacheEntry(remoteRoot+"/"+testDramaName, testSeasonDir)

	remotePath := s.uploadRemoteDirPath(existsPath, remoteRoot, true)

	// 修复前这里得到的是 "/Season.04"（丢失了父级目录 唐朝诡事录 (2022)）
	if want := testDramaName + "/" + testSeasonDir; remotePath != want {
		t.Fatalf("父目录相对路径错误：got %q, want %q", remotePath, want)
	}
	// 上传任务最终使用的目标路径（models.DbUploadTask.RemoteFileId）
	got := filepath.ToSlash(filepath.Join(remoteRoot, remotePath, "tvshow.nfo"))
	want := remoteRoot + "/" + testDramaName + "/" + testSeasonDir + "/tvshow.nfo"
	if got != want {
		t.Fatalf("元数据文件上传路径错误：got %q, want %q", got, want)
	}
}

// 同一次同步中新创建的目录（缓存里保存的是相对网盘根的相对路径）不能重复拼接网盘根
func TestUploadRemoteDirPathLocalDirCreatedInSameSync(t *testing.T) {
	base := t.TempDir()
	remoteRoot := filepath.ToSlash(filepath.Join(base, "CloudDrive2", "115", "媒体库", "电视剧"))
	s := newTestSyncStrm("/strm/电视剧", remoteRoot, remoteRoot)

	// 相对路径形态的缓存记录
	existsPath := newLocalDirCacheEntry(testDramaName, testSeasonDir)

	remotePath := s.uploadRemoteDirPath(existsPath, remoteRoot, true)
	if want := testDramaName + "/" + testSeasonDir; remotePath != want {
		t.Fatalf("父目录相对路径错误：got %q, want %q", remotePath, want)
	}
	got := filepath.ToSlash(filepath.Join(remoteRoot, remotePath, "tvshow.nfo"))
	want := remoteRoot + "/" + testDramaName + "/" + testSeasonDir + "/tvshow.nfo"
	if got != want {
		t.Fatalf("元数据文件上传路径错误：got %q, want %q", got, want)
	}
}

// 115/百度/123 类型的 Path 字段本来就有值，行为必须与修复前完全一致
func TestUploadRemoteDirPathNonLocalUnchanged(t *testing.T) {
	remoteRoot := "/电视剧"
	s := newTestSyncStrm("/strm/电视剧", remoteRoot, "cid-123")

	existsPath := &SyncFileCache{
		Path:       "/" + testDramaName,
		FileName:   testSeasonDir,
		SourceType: models.SourceType115,
	}
	if got, want := s.uploadRemoteDirPath(existsPath, remoteRoot, false), "/"+testDramaName+"/"+testSeasonDir; got != want {
		t.Fatalf("115 类型的父目录路径被改变：got %q, want %q", got, want)
	}
	// 百度网盘/123 网盘的 Path 同样有值，行为不变
	for _, sourceType := range []models.SourceType{models.SourceTypeBaiduPan, models.SourceType123} {
		e := &SyncFileCache{Path: "/" + testDramaName, FileName: testSeasonDir, SourceType: sourceType}
		if got, want := s.uploadRemoteDirPath(e, remoteRoot, false), "/"+testDramaName+"/"+testSeasonDir; got != want {
			t.Fatalf("%s 类型的父目录路径被改变：got %q, want %q", sourceType, got, want)
		}
	}
	// 网盘上不存在父目录时保持原有行为
	if got := s.uploadRemoteDirPath(nil, remoteRoot, false); got != "" {
		t.Fatalf("网盘不存在父目录时应返回空路径：got %q", got)
	}
}

// OpenList 类型的 SyncFileCache.Path 同样从不赋值（真实路径保存在 ParentId 中），
// 修复前用 Path 拼接会退化成 "/Season.04"，把元数据上传到网盘根目录
func TestUploadRemoteDirPathOpenListUsesParentId(t *testing.T) {
	remoteRoot := "/电视剧"
	s := newTestSyncStrm("/strm/电视剧", remoteRoot, remoteRoot)

	existsPath := &SyncFileCache{
		ParentId:   "/" + testDramaName,
		FileName:   testSeasonDir,
		FileType:   v115open.TypeDir,
		SourceType: models.SourceTypeOpenList,
	}

	remotePath := s.uploadRemoteDirPath(existsPath, remoteRoot, false)
	if want := "/" + testDramaName + "/" + testSeasonDir; remotePath != want {
		t.Fatalf("OpenList 类型的父目录路径错误：got %q, want %q", remotePath, want)
	}
	// 非本地类型的 FileId = 父目录路径 + 文件名
	got := filepath.ToSlash(filepath.Join(remotePath, "tvshow.nfo"))
	want := "/" + testDramaName + "/" + testSeasonDir + "/tvshow.nfo"
	if got != want {
		t.Fatalf("OpenList 上传路径错误：got %q, want %q", got, want)
	}
	if got == "/"+testSeasonDir+"/tvshow.nfo" {
		t.Fatalf("OpenList 上传路径丢失了父级目录：got %q", got)
	}
}

// 同步根目录判断：本地类型必须用本地根目录和本地文件路径比较，否则比较恒为真
func TestUploadSyncRoots(t *testing.T) {
	base := t.TempDir()
	localRoot := filepath.Join(base, "strm", "电视剧")
	remoteRoot := filepath.Join(base, "CloudDrive2", "115", "媒体库", "电视剧")
	s := newTestSyncStrm(localRoot, remoteRoot, remoteRoot)

	gotRemoteRoot, gotLocalRoot := s.uploadSyncRoots(true)
	if gotRemoteRoot != filepath.ToSlash(remoteRoot) {
		t.Fatalf("本地类型的网盘根目录错误：got %q, want %q", gotRemoteRoot, filepath.ToSlash(remoteRoot))
	}
	if gotLocalRoot != filepath.ToSlash(localRoot) {
		t.Fatalf("本地类型的本地根目录错误：got %q, want %q", gotLocalRoot, filepath.ToSlash(localRoot))
	}
	// 同步根目录下的文件：filepath.Walk 得到的父目录必须能与本地根目录相等，
	// 否则同步根目录下的元数据文件会走错分支（被当成「网盘不存在父目录」而删除）
	filePath := filepath.ToSlash(filepath.Join(localRoot, "tvshow.nfo"))
	// compareLocalFilesWithTempTable 里对 filepath.Dir 的结果同样会再做一次 ToSlash
	parentDir := filepath.ToSlash(filepath.Dir(filePath))
	if parentDir != gotLocalRoot {
		t.Fatalf("同步根目录下的文件无法识别父目录就是同步根目录：%q != %q", parentDir, gotLocalRoot)
	}

	// 配置里带了结尾分隔符时，根目录判断也必须成立
	withSep := newTestSyncStrm(localRoot+string(os.PathSeparator), remoteRoot, remoteRoot)
	_, localRootWithSep := withSep.uploadSyncRoots(true)
	if localRootWithSep != gotLocalRoot {
		t.Fatalf("本地根目录未做 Clean：got %q, want %q", localRootWithSep, gotLocalRoot)
	}
	if parentDir != localRootWithSep {
		t.Fatalf("带结尾分隔符的配置下，同步根目录下的文件无法识别父目录：%q != %q", parentDir, localRootWithSep)
	}

	// 非本地类型的两个根目录相同，行为与修复前一致
	remoteRoot2, localRoot2 := s.uploadSyncRoots(false)
	want := filepath.ToSlash(filepath.Join(localRoot, remoteRoot))
	if remoteRoot2 != want || localRoot2 != want {
		t.Fatalf("非本地类型的同步根目录错误：got %q/%q, want %q", remoteRoot2, localRoot2, want)
	}
}

// 文件直接位于同步根目录时：本地类型的上传路径相对网盘根就是当前目录
func TestUploadRootParent(t *testing.T) {
	base := t.TempDir()
	remoteRoot := filepath.ToSlash(filepath.Join(base, "CloudDrive2", "115", "媒体库", "电视剧"))
	s := newTestSyncStrm(filepath.Join(base, "strm", "电视剧"), remoteRoot, "cid-123")

	parentPathId, remotePath := s.uploadRootParent(remoteRoot, true)
	if parentPathId != remoteRoot {
		t.Fatalf("本地类型的父目录ID应为网盘根目录：got %q, want %q", parentPathId, remoteRoot)
	}
	if remotePath != "." {
		t.Fatalf("本地类型位于同步根目录下的文件，上传路径应为当前目录：got %q", remotePath)
	}
	got := filepath.ToSlash(filepath.Join(remoteRoot, remotePath, "tvshow.nfo"))
	if want := remoteRoot + "/tvshow.nfo"; got != want {
		t.Fatalf("同步根目录下的文件上传路径错误：got %q, want %q", got, want)
	}

	// 非本地类型保持原有行为
	parentPathId, remotePath = s.uploadRootParent(remoteRoot, false)
	if parentPathId != "cid-123" || remotePath != remoteRoot {
		t.Fatalf("非本地类型的同步根目录父级信息被改变：got %q/%q", parentPathId, remotePath)
	}
}

// 本地类型在同步过程中新建目录后，必须能通过本地路径在缓存里反查到该目录，
// 否则同一次同步中该目录下的元数据文件会因为「父目录不在网盘」而被删除
func TestCreateDirRecursivelyLocalCacheKeepsRemoteParent(t *testing.T) {
	base := t.TempDir()
	localRoot := filepath.Join(base, "strm", "电视剧")
	remoteRoot := filepath.Join(base, "CloudDrive2", "115", "媒体库", "电视剧")
	localDir := filepath.Join(localRoot, testDramaName, testSeasonDir)

	// 本地刮削新增的季目录，网盘（挂载目录）上还不存在
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("创建本地目录失败: %v", err)
	}

	s := newTestSyncStrm(localRoot, remoteRoot, remoteRoot)
	s.memSyncCache = NewMemorySyncCache(1)
	driver := NewLocalDriver()
	driver.SetSyncStrm(s)

	pathId, remotePath, err := driver.CreateDirRecursively(context.Background(), filepath.ToSlash(localDir))
	if err != nil {
		t.Fatalf("创建网盘目录失败: %v", err)
	}
	// 返回的 remotePath 是相对网盘同步根的相对路径，pathId 是网盘上的目录路径
	if want := filepath.ToSlash(filepath.Join(testDramaName, testSeasonDir)); remotePath != want {
		t.Fatalf("创建目录返回的相对路径错误：got %q, want %q", remotePath, want)
	}
	if want := filepath.ToSlash(filepath.Join(remoteRoot, testDramaName, testSeasonDir)); pathId != want {
		t.Fatalf("创建目录返回的网盘路径错误：got %q, want %q", pathId, want)
	}
	if _, err := os.Stat(pathId); err != nil {
		t.Fatalf("网盘目录 %s 未创建成功: %v", pathId, err)
	}

	// 后续文件通过本地路径反查父目录
	entry, err := s.memSyncCache.GetByLocalPath(filepath.ToSlash(localDir))
	if err != nil || entry == nil {
		t.Fatalf("同步缓存中无法用本地路径匹配到新创建的目录 %s: %v", localDir, err)
	}
	// 完整网盘路径必须保留父级目录
	if want := filepath.ToSlash(filepath.Join(remoteRoot, testDramaName, testSeasonDir)); entry.GetFullRemotePath() != want {
		t.Fatalf("新创建目录的网盘完整路径错误：got %q, want %q", entry.GetFullRemotePath(), want)
	}
	// 该目录下元数据文件的上传路径
	got := filepath.ToSlash(filepath.Join(remoteRoot, s.uploadRemoteDirPath(entry, filepath.ToSlash(remoteRoot), true), "tvshow.nfo"))
	want := filepath.ToSlash(filepath.Join(remoteRoot, testDramaName, testSeasonDir, "tvshow.nfo"))
	if got != want {
		t.Fatalf("新创建目录下的元数据文件上传路径错误：got %q, want %q", got, want)
	}
}

// isRelativePathOutsideRoot 不能把 "..config" 这类合法目录名误判为越界
func TestIsRelativePathOutsideRoot(t *testing.T) {
	outside := []string{"..", "../x", `..\x`}
	for _, p := range outside {
		if !isRelativePathOutsideRoot(p) {
			t.Fatalf("%q 应被判断为指向同步根目录之外", p)
		}
	}
	inside := []string{"", ".", "x", "..config", "..config/Season.04", "a/../b"}
	for _, p := range inside {
		if isRelativePathOutsideRoot(p) {
			t.Fatalf("%q 不应被判断为指向同步根目录之外", p)
		}
	}
}

// 通用性验证：与具体剧名无关。遍历各种目录名（空格、括号、中文、日文、井号、加号、
// 以点开头的合法目录名）和 1~6 层嵌套深度，本地类型与 OpenList 类型都必须保留全部层级。
// 期望路径不是写死的字符串，而是按「网盘根 + 各层目录名 + 文件名」现算出来的。
func TestUploadRemoteDirPathGenericShapes(t *testing.T) {
	remoteRoot := filepath.ToSlash(filepath.Join(t.TempDir(), "mnt", "clouddrive", "115", "媒体库"))

	cases := []struct {
		name   string
		levels []string // 从网盘同步根到该父目录的各层目录名
	}{
		{"单层目录", []string{"电影"}},
		{"两层目录", []string{"电视剧", "Season.04"}},
		{"带空格和括号", []string{"电视剧", testDramaName, "Season.04"}},
		{"纯ASCII空格括号", []string{"My Show (2020)", "Season 1"}},
		{"六层嵌套", []string{"A", "B", "C", "D", "E", "F"}},
		{"中文日文混排", []string{"动漫", "進撃の巨人 (2013)", "Season.03"}},
		{"井号加号方括号", []string{"[Group] Show #1 + OVA", "Season.01"}},
		{"以点开头的目录名", []string{"..config", "Season.02"}},
		{"多层且末层带点", []string{"纪录片", "地球脉动 第二季", "Season.1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantRel := filepath.ToSlash(filepath.Join(tc.levels...))
			parentLevels := tc.levels[:len(tc.levels)-1]
			parentRemoteDir := remoteRoot
			if len(parentLevels) > 0 {
				parentRemoteDir = filepath.ToSlash(filepath.Join(remoteRoot, filepath.Join(parentLevels...)))
			}
			lastLevel := tc.levels[len(tc.levels)-1]

			// 本地类型（网盘是挂载到本地的目录）：Path 恒空，真实网盘路径在 ParentId 中
			local := newTestSyncStrm("/strm/电视剧", remoteRoot, remoteRoot)
			localEntry := newLocalDirCacheEntry(parentRemoteDir, lastLevel)
			if got := local.uploadRemoteDirPath(localEntry, remoteRoot, true); got != wantRel {
				t.Fatalf("本地类型：got %q, want %q", got, wantRel)
			}
			gotFile := filepath.ToSlash(filepath.Join(remoteRoot, local.uploadRemoteDirPath(localEntry, remoteRoot, true), "tvshow.nfo"))
			wantFile := filepath.ToSlash(filepath.Join(remoteRoot, wantRel, "tvshow.nfo"))
			if gotFile != wantFile {
				t.Fatalf("本地类型最终上传路径：got %q, want %q", gotFile, wantFile)
			}

			// OpenList 类型：Path 同样从不赋值，真实路径在 ParentId 中
			ol := newTestSyncStrm("/strm/电视剧", remoteRoot, remoteRoot)
			olEntry := &SyncFileCache{
				ParentId:   parentRemoteDir,
				FileName:   lastLevel,
				FileType:   v115open.TypeDir,
				SourceType: models.SourceTypeOpenList,
			}
			wantAbs := filepath.ToSlash(filepath.Join(remoteRoot, wantRel))
			if got := ol.uploadRemoteDirPath(olEntry, remoteRoot, false); got != wantAbs {
				t.Fatalf("OpenList 类型：got %q, want %q", got, wantAbs)
			}

			// 115/百度/123 类型：Path 保存父目录在网盘中的路径，取值必须与修复前一致
			for _, sourceType := range []models.SourceType{models.SourceType115, models.SourceTypeBaiduPan, models.SourceType123} {
				e := &SyncFileCache{Path: parentRemoteDir, FileName: lastLevel, SourceType: sourceType}
				if got := ol.uploadRemoteDirPath(e, remoteRoot, false); got != wantAbs {
					t.Fatalf("%s 类型：got %q, want %q", sourceType, got, wantAbs)
				}
			}
		})
	}
}
