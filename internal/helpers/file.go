package helpers

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func isCrossDeviceError(err error) bool {
	return strings.Contains(err.Error(), "invalid cross-device link") ||
		strings.Contains(err.Error(), "cross-device link")
}

func isDiffrentDriver(err error) bool {
	return strings.Contains(err.Error(), "different disk drive")
}

// PathExists checks if a given path exists.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	// fmt.Printf("检查路径是否存在: %s => %+v, %v\n", path, info, err)
	return err == nil || os.IsExist(err)
}

// 复制文件
func CopyFile(src, dst string) error {
	if PathExists(dst) {
		fmt.Printf("文件已存在，跳过复制: %s\n", dst)
		return nil
	}
	input, err := os.ReadFile(src)
	if err != nil {
		fmt.Printf("读取源文件失败: %s=>%v", src, err)
		return err
	}
	werr := WriteFileWithPerm(dst, input, 0777)
	// dstPath := filepath.Dir(dst)
	// err = CreateDirWithPerm(dstPath, 0777)
	// if err != nil {
	// 	fmt.Printf("创建目标目录失败: %v", err)
	// 	return err
	// }
	// werr := os.WriteFile(dst, input, 0766)
	if werr != nil {
		fmt.Printf("写入目标文件失败: %s=>%v", dst, werr)
		return werr
	}
	return nil
}

// 移动文件
func MoveFile(src, dst string, overwrite bool) error {
	if PathExists(dst) {
		if !overwrite {
			return nil
		}
		os.Remove(dst)
	}
	// Try os.Rename first (will work on same filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// If cross-device error, use copy+delete
	if isCrossDeviceError(err) || isDiffrentDriver(err) {
		if cerr := CopyFile(src, dst); cerr != nil {
			return cerr
		}
		return os.Remove(src)
	}
	return err
}

func WriteJsonFile(filePath string, data interface{}) error {
	jsonData, jsonErr := json.Marshal(data)
	if jsonErr != nil {
		return jsonErr
	}
	return os.WriteFile(filePath, jsonData, 0666)
}

func ReadJsonFile(filePath string, data interface{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(data)
}

// 如果文件存在则备份文件，保留3个旧版本的文件
// 如果备份文件超过3个，则删除最旧的文件直到留下3个备份文件
// 如果文件不存在直接返回
func CheckAndBackupFile(filePath string, maxBackupCount int) {
	if !PathExists(filePath) {
		return
	}
	// 备份文件
	for i := maxBackupCount; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.bak.%d", filePath, i)
		newPath := fmt.Sprintf("%s.bak.%d", filePath, i+1)
		if PathExists(oldPath) {
			os.Rename(oldPath, newPath)
		}
	}
	os.Rename(filePath, fmt.Sprintf("%s.bak.1", filePath))
	// 删除多余的备份文件
	for i := maxBackupCount + 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.bak.%d", filePath, i)
		if PathExists(oldPath) {
			os.Remove(oldPath)
		}
	}
}

func CreateTempDir() string {
	dir, err := os.MkdirTemp("", "test_")
	if err != nil {
		panic(err)
	}
	return dir
}

func RemoveTempDir(dir string) {
	os.RemoveAll(dir)
}

func ReadFileContent(filepath string) string {
	content, err := os.ReadFile(filepath)
	if err != nil {
		fmt.Printf("读取文件内容失败: %s=>%v", filepath, err)
		return ""
	}
	return string(content)
}

func ChangePermissionsWithSudo(path string, mode string) error {
	cmd := exec.Command("sudo", "chmod", mode, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to change permissions: %v, output: %s", err, string(output))
	}
	return nil
}

func ChangeOwnerWithSudo(path, user, group string) error {
	owner := user
	if group != "" {
		owner = user + ":" + group
	}
	cmd := exec.Command("sudo", "chown", owner, path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to change owner: %v, output: %s", err, string(output))
	}
	return nil
}

func IsRunningInDocker() bool {
	// 检查环境变量
	if env := os.Getenv("DOCKER"); env == "1" {
		return true
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// 检查 cgroup
	if content, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		if strings.Contains(string(content), "docker") {
			return true
		}
	}

	return false
}

// MoveDir 将src目录下的所有文件和子文件夹内的文件都移动到dst目录
func MoveDir(src, dst string) error {
	// 检查源目录是否存在
	if !PathExists(src) {
		return fmt.Errorf("源目录不存在: %s", src)
	}

	// 创建目标目录
	err := os.MkdirAll(dst, 0766)
	if err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 遍历源目录下的所有文件和子目录
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过源目录本身
		if path == src {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// 计算目标路径
		dstPath := filepath.Join(dst, relPath)

		// 如果是目录，创建对应的目标目录
		if info.IsDir() {
			return os.MkdirAll(dstPath, 0766)
		}

		// 如果是文件，移动到目标位置
		return MoveFile(path, dstPath, true)
	})
}

// CleanFileName 清理文件名中的非法字符
func CleanFileName(name string) string {
	// Windows 文件系统非法字符
	illegalChars := regexp.MustCompile(`[<>:"/\\|?*]`)

	// 替换非法字符为下划线
	cleaned := illegalChars.ReplaceAllString(name, "_")

	// 移除开头和结尾的点号和空格（Windows 不允许）
	cleaned = strings.Trim(cleaned, ". ")

	// 如果清理后为空，返回默认名称
	if cleaned == "" {
		return "unnamed_file"
	}

	return cleaned
}

// ReadLines 从指定位置读取指定行数，支持向前或向后读取
// direction: "forward" 向前读取, "backward" 向后读取
// 返回：读取到的行，新的读取位置，错误信息
func ReadLines(filename string, startPos int64, maxLines int, direction string) ([]string, int64, error) {
	if direction == "backward" {
		return readLinesBackward(filename, startPos, maxLines)
	}
	return readLinesForward(filename, startPos, maxLines)
}

// 向后读取（原始逻辑）
func readLinesBackward(filename string, startPos int64, maxLines int) ([]string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	fileSize := stat.Size()

	// 如果起始位置超过文件大小，返回空结果
	if startPos >= fileSize {
		return []string{}, fileSize, nil
	}
	if startPos == -1 {
		startPos = 0
	}

	// 设置读取起始位置
	_, err = file.Seek(startPos, io.SeekStart)
	if err != nil {
		return nil, 0, err
	}

	lines := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(file)
	var bytesRead int64

	for scanner.Scan() && len(lines) < maxLines {
		line := scanner.Text()
		lines = append(lines, line)
		// 估算读取的字节数（包括换行符）
		bytesRead += int64(len(line) + 1) // +1 for newline
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	newPosition := min(startPos+bytesRead, fileSize)

	return lines, newPosition, nil
}

// 向前读取
func readLinesForward(filename string, startPos int64, maxLines int) ([]string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	fileSize := stat.Size()

	// 如果起始位置是文件末尾，或者超过文件大小，则从文件末尾开始
	if startPos < 0 || startPos > fileSize {
		startPos = fileSize
	}

	lines := make([]string, 0, maxLines)
	const bufferSize = 64 * 1024 // 64KB 缓冲区
	buf := make([]byte, bufferSize)

	currentPos := startPos
	var totalBytesRead int64

	for currentPos > 0 && len(lines) < maxLines {
		// 计算本次读取的大小
		readSize := int64(bufferSize)
		if currentPos < readSize {
			readSize = currentPos
		}

		// 计算读取位置
		readPos := currentPos - readSize

		// 读取数据块
		n, err := file.ReadAt(buf[:readSize], readPos)
		if err != nil && err != io.EOF {
			return nil, 0, err
		}

		// 在缓冲区中从后往前扫描行
		scanPos := n
		for scanPos > 0 && len(lines) < maxLines {
			// 查找上一个换行符
			newlinePos := bytes.LastIndexByte(buf[:scanPos], '\n')

			if newlinePos == -1 {
				// 没有找到换行符，这是一个长行或者文件开头
				if readPos == 0 {
					// 文件开头，读取剩余内容作为一行
					line := string(buf[:scanPos])
					lines = append([]string{line}, lines...)
					totalBytesRead += int64(scanPos)
					currentPos = 0
					break
				} else {
					// 需要读取更多数据来处理这个长行
					break
				}
			}

			if newlinePos < scanPos-1 {
				// 找到完整的行（跳过空行）
				line := string(buf[newlinePos+1 : scanPos])
				lines = append([]string{line}, lines...)
				totalBytesRead += int64(scanPos - newlinePos - 1)
			} else {
				// 空行，只计入换行符
				totalBytesRead += 1
			}

			scanPos = newlinePos
		}

		currentPos = readPos

		// 如果当前块没有找到足够的行，继续向前读取
		if len(lines) >= maxLines {
			break
		}
	}
	// 翻转lines
	newLines := make([]string, len(lines))
	// 从后往前循环lines
	for i := len(lines) - 1; i >= 0; i-- {
		newLines[len(lines)-1-i] = lines[i]
	}

	// 计算新的读取位置
	newPosition := startPos - totalBytesRead
	if newPosition < 0 {
		newPosition = 0
	}

	return newLines, newPosition, nil
}

// CreateDirWithPerm 创建文件夹，支持传入权限，如果设置了GPID和GUID则改变所有者
func CreateDirWithPerm(dirPath string, perm os.FileMode) error {
	// 创建目录
	if err := os.MkdirAll(dirPath, perm); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %v", dirPath, err)
	}
	return nil
}

// WriteFileWithPerm 写入文件并设置所有者
func WriteFileWithPerm(filePath string, content []byte, perm os.FileMode) error {
	// 先检查路径是否有文件夹，如果有则先创建文件夹
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := CreateDirWithPerm(dir, perm); err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}
	}
	// 写入文件
	err := os.WriteFile(filePath, content, perm)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}
	return nil
}

// 将srcDir下的所有文件复制到destDir下
func CopyDir(srcDir, dstDir string) error {
	// 创建目标目录
	err := os.MkdirAll(dstDir, 0755)
	if err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 遍历源目录
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			// 创建子目录
			return os.MkdirAll(destPath, info.Mode())
		} else {
			// 复制文件
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(destPath, data, info.Mode())
		}
	})
}

type FileChunkMD5Result struct {
	FileMD5          string   `json:"file_md5"`
	ChunkMD5s        []string `json:"chunk_md5s"`
	ChunkSize        int64    `json:"chunk_size"`
	ChunkCount       int      `json:"chunk_count"`
	FileSize         int64    `json:"file_size"`
	ChunkMD5sJsonStr string   `json:"chunk_md5s_json_str"`
}

const (
	DefaultChunkSize = 4 * 1024 * 1024
	MaxChunkCount    = 1024
)

func CalculateFileChunkMD5(filePath string, chunkSize int64) (*FileChunkMD5Result, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}
	fileSize := fileInfo.Size()

	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	if fileSize <= chunkSize {
		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			return nil, fmt.Errorf("计算文件MD5失败: %v", err)
		}
		fileMD5 := hex.EncodeToString(hash.Sum(nil))
		return &FileChunkMD5Result{
			FileMD5:    fileMD5,
			ChunkMD5s:  []string{fileMD5},
			ChunkSize:  fileSize,
			ChunkCount: 1,
			FileSize:   fileSize,
		}, nil
	}

	expectedChunks := int((fileSize + chunkSize - 1) / chunkSize)
	if expectedChunks > MaxChunkCount {
		chunkSize = (fileSize + int64(MaxChunkCount) - 1) / int64(MaxChunkCount)
	}

	var chunkMD5s []string
	fileHash := md5.New()
	buf := make([]byte, chunkSize)
	position := int64(0)

	for position < fileSize {
		remaining := fileSize - position
		readSize := chunkSize
		if remaining < chunkSize {
			readSize = remaining
		}

		n, err := file.ReadAt(buf[:readSize], position)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("读取文件分片失败: %v", err)
		}

		fileHash.Write(buf[:n])

		chunkHash := md5.New()
		chunkHash.Write(buf[:n])
		chunkMD5s = append(chunkMD5s, hex.EncodeToString(chunkHash.Sum(nil)))

		position += int64(n)
	}

	return &FileChunkMD5Result{
		FileMD5:    hex.EncodeToString(fileHash.Sum(nil)),
		ChunkMD5s:  chunkMD5s,
		ChunkSize:  chunkSize,
		ChunkCount: len(chunkMD5s),
		FileSize:   fileSize,
	}, nil
}

const (
	PartialMD5Size = 256 * 1024
)

func CalculateFilePartialMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %v", err)
	}
	fileSize := fileInfo.Size()

	hash := md5.New()

	if fileSize <= PartialMD5Size {
		if _, err := io.Copy(hash, file); err != nil {
			return "", fmt.Errorf("计算文件MD5失败: %v", err)
		}
	} else {
		buf := make([]byte, PartialMD5Size)
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("读取文件失败: %v", err)
		}
		hash.Write(buf[:n])
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func ExtractFileChunkToTemp(filePath string, chunkSize int64, chunkIndex int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %v", err)
	}
	fileSize := fileInfo.Size()

	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	if chunkIndex < 0 {
		return "", fmt.Errorf("分片序号无效: %d", chunkIndex)
	}

	offset := int64(chunkIndex) * chunkSize
	if offset >= fileSize {
		return "", fmt.Errorf("分片序号超出文件范围: %d", chunkIndex)
	}

	remaining := fileSize - offset
	readSize := chunkSize
	if remaining < chunkSize {
		readSize = remaining
	}

	buf := make([]byte, readSize)
	n, err := file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("读取文件分片失败: %v", err)
	}

	tempFile, err := os.CreateTemp("", "chunk_*")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.Write(buf[:n]); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}

	return tempFile.Name(), nil
}

func IsDirEmpty(dirPath string) bool {
	dir, err := os.Open(dirPath)
	if err != nil {
		return false
	}
	defer dir.Close()

	// 读取目录下的所有文件
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return false
	}

	// 如果目录下没有文件或子目录，返回true
	return len(names) == 0
}
