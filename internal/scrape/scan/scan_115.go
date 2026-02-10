package scan

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// 从115扫描需要刮削的文件入库
type Scan115Impl struct {
	scanBaseImpl
	client *v115open.OpenClient
}

func New115ScanImpl(scrapePath *models.ScrapePath, client *v115open.OpenClient, ctx context.Context) *Scan115Impl {
	return &Scan115Impl{scanBaseImpl: scanBaseImpl{ctx: ctx, scrapePath: scrapePath}, client: client}
}

// 检查来源目录和目标目录是否存在
func (s *Scan115Impl) CheckPathExists() error {
	// 检查sourceId是否存在
	sourceDetail, err := s.client.GetFsDetailByCid(s.ctx, s.scrapePath.SourcePathId)
	if err != nil || sourceDetail.FileId == "" {
		ferr := fmt.Errorf("刮削来源目录 %s => %s 疑似不存在，请检查或编辑重新选择来源目录: %v", s.scrapePath.SourcePathId, s.scrapePath.SourcePath, err)
		return ferr
	}
	if s.scrapePath.ScrapeType != models.ScrapeTypeOnly {
		// 检查targetId是否存在
		targetDetail, err := s.client.GetFsDetailByCid(s.ctx, s.scrapePath.DestPathId)
		if err != nil || targetDetail.FileId == "" {
			ferr := fmt.Errorf("刮削目标目录 %s => %s 疑似不存在，请检查或编辑重新选择目标目录: %v", s.scrapePath.DestPathId, s.scrapePath.DestPath, err)
			return ferr
		}
	}
	return nil
}

// 扫描网盘文件
// 递归扫描指定路径下的所有文件
// 返回收集到的待刮削文件总数
func (s *Scan115Impl) GetNetFileFiles() error {
	// 批次号
	s.BatchNo = time.Now().Format("20060102150405000")
	// 检查是否停止任务
	if !s.CheckIsRunning() {
		return errors.New("任务已停止")
	}
	// 检查sourceId是否存在
	sourceDetail, err := s.client.GetFsDetailByCid(s.ctx, s.scrapePath.SourcePathId)
	if err != nil || sourceDetail.FileId == "" {
		ferr := fmt.Errorf("刮削目录 %s => %s 疑似不存在，请检查或编辑重新选择来源目录: %v", s.scrapePath.SourcePathId, s.scrapePath.SourcePath, err)
		return ferr
	}
	// 初始化路径队列，容量为接口线程数
	s.pathTasks = make(chan string, models.SettingsGlobal.FileDetailThreads)
	// 启动一个控制buffer的context
	bufferCtx, cancelBuffer := context.WithCancel(context.Background())
	// 启动buffer to task
	go s.bufferMonitor(bufferCtx)
	// 加入根目录
	s.wg = sync.WaitGroup{}
	s.addPathToTasks(s.scrapePath.SourcePathId)
	// 启动一个协程处理目录
	helpers.AppLogger.Infof("开始处理目录 %s, 开启 %d 个任务", s.scrapePath.SourcePath, models.SettingsGlobal.FileDetailThreads)
	for i := 0; i < models.SettingsGlobal.FileDetailThreads; i++ {
		// 在限速器控制下执行StartPathWork
		go s.startPathWorkWithLimiter(i)
	}
	go func() {
		<-s.ctx.Done()
		for range s.pathTasks {
			s.wg.Done()
		}
	}()
	s.wg.Wait()        // 等待最后一个目录处理完
	close(s.pathTasks) // 关闭pathTasks，释放资源
	cancelBuffer()     // 取消bufferMonitor上下文，释放资源
	return nil
}

func (s *Scan115Impl) startPathWorkWithLimiter(workerID int) {
	// 从channel获取路径任务
	for {
		select {
		case <-s.ctx.Done():
			return
		case pathId, ok := <-s.pathTasks:
			if !ok {
				return
			}
			offset := 0
			limit := 1150
			// 记录下图片、nfo、字幕文件
			// 如果发现了视频文件，则寻找有没有视频文件对应的图片、nfo、字幕文件
			// 如果没发现视频文件，则清空
			picFiles := make([]*localFile, 0)
			nfoFiles := make([]*localFile, 0)
			subFiles := make([]*localFile, 0)
			videoFiles := make([]*localFile, 0)
			parentPath := ""
		pageloop:
			for {
				// 分页取文件夹内容
				// 查询目录下所有文件和文件夹
				helpers.AppLogger.Infof("worker %d 开始处理目录 %s, offset=%d, limit=%d", workerID, pathId, offset, limit)
				fsList, err := s.client.GetFsList(s.ctx, pathId, true, false, true, offset, limit)
				if err != nil {
					if strings.Contains(err.Error(), "context canceled") {
						s.wg.Done()
						helpers.AppLogger.Infof("worker %d 处理目录 %s 失败 上下文已取消", workerID, pathId)
						return
					}
					helpers.AppLogger.Errorf("worker %d 处理目录 %s 失败: %v", workerID, pathId, err)
					continue pageloop
				}
				parentPath = fsList.PathStr
				// 取完就跳出
				if len(fsList.Data) == 0 {
					break pageloop
				}
			fileloop:
				for _, file := range fsList.Data {
					if !s.CheckIsRunning() {
						s.wg.Done()
						return
					}
					if file.FileCategory == v115open.TypeDir {
						// 是目录，加入队列
						s.addPathToTasks(file.FileId)
						continue fileloop
					}
					if file.Aid != "1" {
						helpers.AppLogger.Infof("文件 %s 已放入回收站或删除，跳过", file.FileName)
						continue
					}
					// 检查文件是否允许处理
					if !s.scrapePath.CheckFileIsAllowed(file.FileName, file.FileSize) {
						continue
					}
					// 检查文件是否已处理，如果已在数据库中，则直接跳过
					exists := models.CheckExistsFileIdAndName(file.FileId, s.scrapePath.ID)
					if exists {
						helpers.AppLogger.Infof("文件 %s 已在数据库中，跳过", file.FileName)
						continue
					}
					ext := filepath.Ext(file.FileName)
					helpers.AppLogger.Infof("开始处理文件 %s 扩展名 %s", file.FileName, ext)
					if slices.Contains(models.SubtitleExtArr, ext) {
						helpers.AppLogger.Infof("文件 %s 是字幕文件", file.FileName)
						subFiles = append(subFiles, &localFile{
							Id:       file.FileId,
							PickCode: file.PickCode,
							Name:     file.FileName,
							Size:     file.FileSize,
							Path:     filepath.Join(parentPath, file.FileName),
						})
						continue fileloop
					}
					if slices.Contains(models.ImageExtArr, ext) {
						helpers.AppLogger.Infof("文件 %s 是图片文件", file.FileName)
						picFiles = append(picFiles, &localFile{
							Id:       file.FileId,
							PickCode: file.PickCode,
							Name:     file.FileName,
							Size:     file.FileSize,
							Path:     filepath.Join(parentPath, file.FileName),
						})
						continue fileloop
					}
					if ext == ".nfo" {
						helpers.AppLogger.Infof("文件 %s 是nfo文件", file.FileName)
						nfoFiles = append(nfoFiles, &localFile{
							Id:       file.FileId,
							PickCode: file.PickCode,
							Name:     file.FileName,
							Size:     file.FileSize,
							Path:     filepath.Join(parentPath, file.FileName),
						})
						continue fileloop
					}
					if s.scrapePath.IsVideoFile(file.FileName) {
						videoFiles = append(videoFiles, &localFile{
							Id:       file.FileId,
							PickCode: file.PickCode,
							Name:     file.FileName,
							Size:     file.FileSize,
							Path:     filepath.Join(parentPath, file.FileName),
						})
						continue fileloop
					}
				}
				offset += limit
				if offset >= fsList.Count {
					break pageloop
				}
			}
			helpers.AppLogger.Infof("目录 %s 处理完成，%d 个视频文件，%d 个图片文件，%d 个nfo文件，%d 个字幕文件", parentPath, len(videoFiles), len(picFiles), len(nfoFiles), len(subFiles))
			// 处理视频文件
			verr := s.processVideoFile(parentPath, pathId, videoFiles, picFiles, nfoFiles, subFiles)
			if verr != nil {
				s.wg.Done()
				return
			}
			// 任务完成，通知WaitGroup
			s.wg.Done()
		}
	}
}
