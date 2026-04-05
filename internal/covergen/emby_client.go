package covergen

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	embyclient "Q115-STRM/internal/embyclient-rest-go"
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type EmbyImageClient struct {
	client   *embyclient.Client
	embyURL  string
	apiKey   string
	httpClient *http.Client
}

func NewEmbyImageClient() (*EmbyImageClient, error) {
	embyConfig, err := models.GetEmbyConfig()
	if err != nil {
		return nil, fmt.Errorf("获取Emby配置失败: %w", err)
	}
	
	if embyConfig.EmbyUrl == "" || embyConfig.EmbyApiKey == "" {
		return nil, fmt.Errorf("Emby未配置")
	}
	
	client := embyclient.NewClient(embyConfig.EmbyUrl, embyConfig.EmbyApiKey)
	
	return &EmbyImageClient{
		client:   client,
		embyURL:  embyConfig.EmbyUrl,
		apiKey:   embyConfig.EmbyApiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type MediaItemImage struct {
	ItemID   string
	ImageURL string
	ImageType string
	LocalPath string
}

func (c *EmbyImageClient) GetLibraryMediaItems(libraryID string, sortBy CoverSort, limit int) ([]embyclient.BaseItemDtoV2, error) {
	allItems, err := c.client.GetMediaItemsByLibraryID(libraryID, 0)
	if err != nil {
		return nil, fmt.Errorf("获取媒体项失败: %w", err)
	}
	
	items := allItems
	
	if sortBy == CoverSortRandom {
		rand.Shuffle(len(items), func(i, j int) {
			items[i], items[j] = items[j], items[i]
		})
	}
	
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	
	return items, nil
}

func (c *EmbyImageClient) DownloadItemImage(itemID string, usePrimary bool, tempDir string) (*MediaItemImage, error) {
	imageType := "Backdrop"
	if usePrimary {
		imageType = "Primary"
	}
	
	url := fmt.Sprintf("%s/emby/Items/%s/Images/%s?api_key=%s", c.embyURL, itemID, imageType, c.apiKey)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("获取图片失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		if usePrimary {
			return c.DownloadItemImage(itemID, false, tempDir)
		}
		return nil, fmt.Errorf("获取图片失败，状态码: %d", resp.StatusCode)
	}
	
	ext := ".jpg"
	contentType := resp.Header.Get("Content-Type")
	if contentType == "image/png" {
		ext = ".png"
	}
	
	filename := fmt.Sprintf("%s_%s%s", itemID, imageType, ext)
	localPath := filepath.Join(tempDir, filename)
	
	file, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("创建图片文件失败: %w", err)
	}
	defer file.Close()
	
	if _, err := io.Copy(file, resp.Body); err != nil {
		return nil, fmt.Errorf("保存图片失败: %w", err)
	}
	
	return &MediaItemImage{
		ItemID:    itemID,
		ImageURL:  url,
		ImageType: imageType,
		LocalPath: localPath,
	}, nil
}

func (c *EmbyImageClient) UploadItemImage(itemId string, imageData []byte, contentType string) error {
	url := fmt.Sprintf("%s/emby/Items/%s/Images/Primary?api_key=%s", c.embyURL, itemId, c.apiKey)
	
	req, err := http.NewRequest("POST", url, bytes.NewReader(imageData))
	if err != nil {
		return fmt.Errorf("创建上传请求失败: %w", err)
	}
	
	req.Header.Set("Content-Type", contentType)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("上传图片失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("上传图片失败，状态码: %d", resp.StatusCode)
	}
	
	return nil
}

func (c *EmbyImageClient) DownloadImagesForLibrary(libraryID string, count int, usePrimary bool, sortBy CoverSort) ([]*MediaItemImage, error) {
	items, err := c.GetLibraryMediaItems(libraryID, sortBy, count)
	if err != nil {
		return nil, err
	}
	
	if len(items) == 0 {
		return nil, fmt.Errorf("媒体库中没有媒体项")
	}
	
	tempDir := filepath.Join(helpers.ConfigDir, "tmp", "covergen")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	
	images := make([]*MediaItemImage, 0, count)
	for i, item := range items {
		if i >= count {
			break
		}
		
		image, err := c.DownloadItemImage(item.Id, usePrimary, tempDir)
		if err != nil {
			helpers.AppLogger.Warnf("下载媒体项 %s 的图片失败: %v", item.Id, err)
			continue
		}
		images = append(images, image)
	}
	
	if len(images) == 0 {
		return nil, fmt.Errorf("没有成功下载任何图片")
	}
	
	return images, nil
}

func (c *EmbyImageClient) UploadCoverToEmby(libraryID string, imgData []byte) error {
	libraries, err := c.client.GetAllMediaLibraries()
	if err != nil {
		return fmt.Errorf("获取媒体库列表失败: %w", err)
	}
	
	var targetLibraryID string
	for _, lib := range libraries {
		if lib.ID == libraryID {
			targetLibraryID = lib.ID
			break
		}
	}
	
	if targetLibraryID == "" {
		return fmt.Errorf("未找到媒体库: %s", libraryID)
	}
	
	contentType := "image/jpeg"
	return c.UploadItemImage(targetLibraryID, imgData, contentType)
}

func SaveImageAsJPEG(imgData []byte, path string) error {
	return os.WriteFile(path, imgData, 0644)
}

func LoadImageFromJPEG(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func CleanupTempImages(images []*MediaItemImage) {
	for _, img := range images {
		if img.LocalPath != "" {
			if err := os.Remove(img.LocalPath); err != nil {
				helpers.AppLogger.Warnf("删除临时图片失败: %s, 错误: %v", img.LocalPath, err)
			}
		}
	}
}

func EncodeImageToJPEG(imgData []byte, quality int) ([]byte, error) {
	return imgData, nil
}

func DecodeImageFromJPEG(data []byte) ([]byte, error) {
	return data, nil
}
