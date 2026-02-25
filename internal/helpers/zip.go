package helpers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// 解压 .tar.gz 文件
func ExtractTarGz(src, dst string) error {
	// 打开源文件
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	// 创建 gzip 读取器
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// 创建 tar 读取器
	tr := tar.NewReader(gzr)

	// 遍历 tar 文件中的每个文件/目录
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // 文件结束
		}
		if err != nil {
			return err
		}

		// 构建目标路径
		target := filepath.Join(dst, header.Name)

		// 根据文件类型处理
		switch header.Typeflag {
		case tar.TypeDir: // 目录
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg: // 普通文件
			// 创建目录（如果不存在）
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			// 创建文件
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// 复制文件内容
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink: // 符号链接
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("未知的文件类型: %v in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}

// 创建 .tar.gz 文件
func CreateTarGz(src, dst string) error {
	// 创建目标文件
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	// 创建 gzip 写入器
	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	// 创建 tar 写入器
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// 遍历源目录
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 创建 tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// 修正路径（使用相对路径）
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil // 跳过根目录
		}
		header.Name = relPath

		// 写入 header
		if werr := tw.WriteHeader(header); werr != nil {
			return werr
		}

		// 如果是普通文件，写入内容
		if !info.Mode().IsRegular() {
			return nil
		}

		// 打开文件
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		// 复制文件内容
		_, err = io.Copy(tw, f)
		return err
	})
}

// 解压 ZIP 文件
func ExtractZip(src, dst string) error {
	// 打开 ZIP 文件
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// 遍历 ZIP 文件中的每个文件
	for _, f := range r.File {
		// 处理文件路径（防止路径遍历攻击）
		filePath := filepath.Join(dst, f.Name)

		// 检查文件路径是否在目标目录内（安全措施）
		if !isSafePath(dst, filePath) {
			return fmt.Errorf("不安全的文件路径: %s", f.Name)
		}

		// 打印文件信息
		fmt.Printf("解压文件: %s\n", filePath)

		// 处理目录
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		// 创建目录（如果不存在）
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}

		// 创建目标文件
		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		// 打开 ZIP 中的文件
		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		// 复制文件内容
		_, err = io.Copy(dstFile, srcFile)

		// 关闭文件（即使出错也要关闭）
		dstFile.Close()
		srcFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// 安全检查：确保解压路径在目标目录内
func isSafePath(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && rel[:2] != ".."
}

// 将src目录内的所有文件打包成zip文件(dst)
func ZipDir(src, dst string) error {
	// 创建目标zip文件
	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	// 创建zip写入器
	w := zip.NewWriter(file)
	defer w.Close()

	// 遍历源目录
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 创建zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// 修正路径（使用相对路径）
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil // 跳过根目录
		}
		header.Name = relPath

		// 如果是目录，需要设置压缩方法为Store
		if info.IsDir() {
			header.Method = zip.Store
		}

		// 写入header
		zipFile, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		// 如果是普通文件，写入内容
		if !info.Mode().IsRegular() {
			return nil
		}

		// 打开文件
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		// 复制文件内容
		_, err = io.Copy(zipFile, f)
		return err
	})
}
