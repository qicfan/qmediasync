package scrape

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

// 识别电视剧

type IdTvShowImpl struct {
	IdBase
}

func NewIdTvShowImpl(scrapePath *models.ScrapePath, ctx context.Context, tmdbImpl *TmdbTvShowImpl) *IdTvShowImpl {
	return &IdTvShowImpl{
		IdBase: IdBase{
			tmdbImpl:   tmdbImpl,
			scrapePath: scrapePath,
			ctx:        ctx,
		},
	}
}

// 识别电视剧
func (i *IdTvShowImpl) Identify(mediaFile *models.ScrapeMediaFile) error {
	// 如果有了tmdbid不用继续识别，重新识别会自动填入tmdbid
	if mediaFile.IsReScrape || mediaFile.TmdbId != 0 {
		return nil
	}
	// 从文件名和文件夹名中提取信息
	info, err := i.extractInfo(mediaFile)
	if info != nil && info.Name != "" {
		mediaFile.Name = helpers.CleanFileName(info.Name)
		mediaFile.Year = info.Year
		mediaFile.Save()
	}
	if err != nil {
		reason := err.Error()
		if err.Error() == "多条记录" {
			reason = "通过名称和年份查询到多部电视剧，需要手工重新识别输入确定的tmdb id"
		}
		mediaFile.Failed(reason)
		return err
	}
	if info.Name == "" || info.Year == 0 || info.TmdbId == 0 {
		return errors.New("无法从文件名和文件夹名中提取到完整的电视剧信息")
	}
	// 保存
	mediaFile.TmdbId = info.TmdbId
	mediaFile.Save()
	return nil
}

func (i *IdTvShowImpl) extractInfo(mediaFile *models.ScrapeMediaFile) (*helpers.MediaInfo, error) {
	// 先从文件名中提取季和集
	disableAI := false
	if i.scrapePath.EnableAi == models.AiActionEnforce {
		// 强制使用AI，如果AI失败则退回正则识别
		disableAI = true
		aiInfo, err := i.extractInfoByAI(mediaFile)
		if err != nil {
			helpers.AppLogger.Errorf("使用AI提取电视剧信息失败，退回正则识别: %v", err)
		} else if aiInfo != nil {
			helpers.AppLogger.Infof("强制使用AI提取电视剧信息成功, 文件名 %s, 文件夹：%s 提取结果 %+v", mediaFile.VideoFilename, filepath.Base(mediaFile.TvshowPath), aiInfo)
			return &helpers.MediaInfo{
				Name:    aiInfo.Name,
				Year:    aiInfo.Year,
				TmdbId:  aiInfo.TmdbId,
				Season:  aiInfo.Season,
				Episode: aiInfo.Episode,
			}, nil
		}
	}
	info, err := i.extractInfoByRE(mediaFile)
	if err != nil {
		helpers.AppLogger.Errorf("使用正则提取电视剧信息失败: %v", err)
		if disableAI || i.scrapePath.EnableAi == models.AiActionOff {
			helpers.AppLogger.Errorf("由于禁用AI识别，所以直接返回错误: %v", err)
			return nil, err
		}
	} else if info.TmdbId != 0 {
		helpers.AppLogger.Infof("使用正则提取电视剧信息成功, 文件名 %s, 文件夹：%s 提取结果 %+v", mediaFile.VideoFilename, filepath.Base(mediaFile.TvshowPath), info)
		return &helpers.MediaInfo{
			Name:   info.Name,
			Year:   info.Year,
			TmdbId: info.TmdbId,
		}, nil
	}
	// 使用AI辅助查询
	assistAiInfo, assistErr := i.extractInfoByAI(mediaFile)
	if assistErr != nil {
		helpers.AppLogger.Errorf("使用AI辅助提取电影信息失败，退回正则识别: %v", assistErr)
		return nil, assistErr
	} else if assistAiInfo != nil {
		return &helpers.MediaInfo{
			Name:   assistAiInfo.Name,
			Year:   assistAiInfo.Year,
			TmdbId: assistAiInfo.TmdbId,
		}, nil
	}
	return nil, errors.New("使用AI辅助从文件名中提取媒体信息查询名称和年份失败, 未找到匹配记录")
}

// AI提取
func (i *IdTvShowImpl) extractInfoByAI(mediaFile *models.ScrapeMediaFile) (*helpers.MediaInfo, error) {
	client := models.GlobalScrapeSettings.GetAiClient()
	info, err := client.TakeMoiveName(mediaFile.VideoFilename, i.scrapePath.GetAiPrompt())
	if err != nil {
		helpers.AppLogger.Errorf("强制使用AI从文件名中提取媒体信息失败: %v", err)
		return nil, err
	}
	if info.Name != "" && info.Year != 0 {
		// 查询
		name, checkId, cyear, cerr := i.tmdbImpl.CheckByNameAndYear(info.Name, info.Year, true)
		if cerr != nil {
			helpers.AppLogger.Errorf("AI从文件名中查询名称和年份失败, 文件名 %s, 提取结果 %+v, 错误信息 %v", mediaFile.VideoFilename, info, cerr)
		}
		if checkId > 0 {
			helpers.AppLogger.Infof("AI从文件名中查询名称和年份成功, 文件名 %s, 提取结果 %+v, 名称：%s, TMDB ID %d", mediaFile.VideoFilename, info, name, checkId)
			// 查到了，直接用
			return &helpers.MediaInfo{
				Name:   name,
				Year:   cyear,
				TmdbId: checkId,
			}, nil
		}
	}
	folderName := filepath.Base(mediaFile.TvshowPath)
	// 从文件夹中提取信息
	helpers.AppLogger.Warnf("AI从文件名中提取媒体信息不全，继续从文件夹中补齐信息，文件名 %s， 提取结果 %+v", mediaFile.VideoFilename, info)
	folderInfo, err := client.TakeMoiveName(folderName, i.scrapePath.GetAiPrompt())
	if err != nil {
		helpers.AppLogger.Errorf("AI从文件夹中提取媒体信息失败, 文件夹 %s, 错误信息 %v", folderName, err)
		return nil, err
	}
	helpers.AppLogger.Infof("AI从文件夹中提取媒体信息，文件夹 %s， 提取结果 %+v", folderName, folderInfo)
	if folderInfo.Year != 0 {
		info.Year = folderInfo.Year
	}
	if folderInfo.Name != "" && info.Name == "" {
		info.Name = folderInfo.Name
	}
	// 使用提取结果查询
	name, checkId, cyear, err := i.tmdbImpl.CheckByNameAndYear(info.Name, info.Year, true)
	if err != nil {
		helpers.AppLogger.Errorf("AI从文件夹中提取媒体信息查询名称和年份失败, 文件夹 %s, 提取结果 %+v, 错误信息 %v", folderName, folderInfo, err)
		return nil, err
	}
	if checkId > 0 {
		helpers.AppLogger.Infof("AI从文件夹中提取媒体信息查询名称和年份成功, 文件夹 %s, 提取结果 %+v, 名称：%s, TMDB ID %d", folderName, folderInfo, name, checkId)
		// 查到了，直接用
		return &helpers.MediaInfo{
			Name:   name,
			Year:   cyear,
			TmdbId: checkId,
		}, nil
	}
	return nil, errors.New("AI从文件夹中提取媒体信息查询名称和年份失败, 未找到匹配记录")
}

// 正则提取
// 从文件名中获取名字+年份
func (i *IdTvShowImpl) extractInfoByRE(mediaFile *models.ScrapeMediaFile) (*helpers.MediaInfo, error) {
	return i.extractInfoByREV2(mediaFile)
	// folderName := filepath.Base(mediaFile.TvshowPath)
	// filename := filepath.Base(mediaFile.VideoFilename)
	// // 从文件名中获取媒体信息
	// info := helpers.ExtractMediaInfoRe(filename, false, false, i.scrapePath.VideoExtList, i.scrapePath.DeleteKeyword...)
	// if info.TmdbId != 0 {
	// 	// 使用tmdb id查询
	// 	cname, cyear, cerr := i.tmdbImpl.CheckByTmdbId(info.TmdbId)
	// 	if cerr != nil {
	// 		helpers.AppLogger.Errorf("使用tmdb id查询媒体信息失败, tmdb id %d, 错误信息 %v", info.TmdbId, cerr)
	// 	} else {
	// 		if cname != "" {
	// 			info.Name = cname
	// 		}
	// 		if cyear != 0 {
	// 			info.Year = cyear
	// 		}
	// 		return info, nil
	// 	}
	// }
	// if info.Name != "" && info.Year != 0 {
	// 	// 使用名称和年份查询
	// 	cname, cid, cyear, cerr := i.tmdbImpl.CheckByNameAndYear(info.Name, info.Year, true)
	// 	if cerr != nil {
	// 		helpers.AppLogger.Errorf("使用名称和年份查询媒体信息失败, 名称 %s, 年份 %d, 错误信息 %v", info.Name, info.Year, cerr)
	// 	} else {
	// 		info.TmdbId = cid
	// 		info.Year = cyear
	// 		info.Name = cname
	// 		helpers.AppLogger.Infof("使用正则从文件名中提取媒体信息成功，文件名 %s, 提取结果 %+v", filename, info)
	// 		return info, nil
	// 	}
	// }
	// helpers.AppLogger.Warnf("文件名 %s, 缺少名称或年份，继续从文件夹中提取", filename)
	// // 使用正则从文件夹中提取信息
	// folderInfo := helpers.ExtractMediaInfoRe(folderName, true, false, i.scrapePath.VideoExtList, i.scrapePath.DeleteKeyword...)
	// helpers.AppLogger.Infof("正则从文件夹中提取信息，文件夹 %s， 提取结果 %+v", folderName, folderInfo)
	// if folderInfo.TmdbId != 0 && info.TmdbId == 0 {
	// 	info.TmdbId = folderInfo.TmdbId
	// }
	// if folderInfo.Name != "" && info.Name == "" {
	// 	info.Name = folderInfo.Name
	// }
	// if folderInfo.Year != 0 {
	// 	info.Year = folderInfo.Year
	// }
	// // 从tmdb查询
	// if info.TmdbId != 0 {
	// 	// 使用tmdb id查询
	// 	cname, cyear, cerr := i.tmdbImpl.CheckByTmdbId(info.TmdbId)
	// 	if cerr != nil {
	// 		helpers.AppLogger.Errorf("使用tmdb id查询媒体信息失败, tmdb id %d, 错误信息 %v", info.TmdbId, cerr)
	// 	} else {
	// 		info.Name = cname
	// 		info.Year = cyear
	// 		helpers.AppLogger.Infof("使用tmdb id查询媒体信息成功, tmdb id %d, 名称 %s, 年份 %d", info.TmdbId, info.Name, info.Year)
	// 		return info, nil
	// 	}
	// }
	// if info.Name != "" {
	// 	// 使用名称和年份查询
	// 	cname, cid, cyear, cerr := i.tmdbImpl.CheckByNameAndYear(info.Name, info.Year, true)
	// 	if cerr != nil {
	// 		err := fmt.Errorf("使用名称和年份查询媒体信息失败, 名称 %s, 年份 %d, 错误信息 %v", info.Name, info.Year, cerr)
	// 		helpers.AppLogger.Errorf(err.Error())
	// 		return nil, err
	// 	} else {
	// 		info.TmdbId = cid
	// 		info.Year = cyear
	// 		info.Name = cname
	// 		helpers.AppLogger.Infof("使用名称和年份查询媒体信息成功, 名称 %s, 年份 %d, 名称: %s TMDB ID %d", info.Name, info.Year, info.Name, info.TmdbId)
	// 		return info, nil
	// 	}
	// }
	// return nil, fmt.Errorf("文件名 %s, 无法提取到任何媒体信息", filename)
}

// 先从文件名中提取名称 + 年份; 如果有直接用，如果没有直接用名称搜索，如果有再使用结果搜索季，如果季的年份=文件名的年份则直接用
// 如果上述无法完成则从文件夹中提取名称+年份；如果有直接用
func (i *IdTvShowImpl) extractInfoByREV2(mediaFile *models.ScrapeMediaFile) (*helpers.MediaInfo, error) {
	folderName := filepath.Base(mediaFile.TvshowPath)
	filename := filepath.Base(mediaFile.VideoFilename)
	// 从文件名中获取媒体信息
	info := helpers.ExtractMediaInfoRe(filename, false, false, i.scrapePath.VideoExtList, i.scrapePath.DeleteKeyword...)
	helpers.AppLogger.Infof("正则从文件名中提取信息，文件名 %s， 提取结果 %+v", filename, info)
	info, err := i.find(mediaFile, info.TmdbId, info.Name, info.Year)
	if err == nil {
		return info, nil
	}
	// 从文件夹中提取信息
	folderInfo := helpers.ExtractMediaInfoRe(folderName, true, false, i.scrapePath.VideoExtList, i.scrapePath.DeleteKeyword...)
	helpers.AppLogger.Infof("正则从文件夹中提取信息，文件夹 %s， 提取结果 %+v", folderName, folderInfo)
	newInfo, ferr := i.find(mediaFile, folderInfo.TmdbId, folderInfo.Name, folderInfo.Year)
	if ferr == nil {
		return newInfo, nil
	}
	return nil, ferr
}

// 从tmdb查询名称和年份是否匹配
func (i *IdTvShowImpl) find(mediaFile *models.ScrapeMediaFile, tmdbid int64, name string, year int) (*helpers.MediaInfo, error) {
	if tmdbid != 0 {
		// 使用tmdb id查询
		cname, cyear, cerr := i.tmdbImpl.CheckByTmdbId(tmdbid)
		if cerr != nil {
			helpers.AppLogger.Errorf("使用tmdb id查询媒体信息失败, tmdb id %d, 错误信息 %v", tmdbid, cerr)
		} else {
			if cname != "" {
				name = cname
			}
			if cyear != 0 {
				year = cyear
			}
			return &helpers.MediaInfo{
				TmdbId: tmdbid,
				Name:   name,
				Year:   year,
			}, nil
		}
	}
	if name != "" && year != 0 {
		// 使用名称和年份查询
		cname, cid, cyear, cerr := i.tmdbImpl.CheckByNameAndYear(name, year, true)
		if cerr != nil {
			helpers.AppLogger.Errorf("使用名称和年份查询电视剧失败, 名称 %s, 年份 %d, 错误信息 %v", name, year, cerr)
		} else {
			return &helpers.MediaInfo{
				TmdbId: cid,
				Name:   cname,
				Year:   cyear,
			}, nil
		}
	}
	if name != "" {
		// 使用名称查询
		cname, cid, cyear, cerr := i.tmdbImpl.CheckByNameAndYear(name, 0, false)
		if cerr != nil {
			helpers.AppLogger.Errorf("使用名称查询电视剧失败, 名称 %s, 错误信息 %v", name, cerr)
			return nil, cerr
		}
		// 继续查询季，确定年份是否正确
		if year != 0 && mediaFile.SeasonNumber >= 0 {
			// 检查季是否存在
			season, cerr := i.tmdbImpl.CheckSeasonByTmdbId(cid, mediaFile.SeasonNumber)
			if cerr != nil {
				helpers.AppLogger.Errorf("使用tmdb id查询季失败, tmdb id %d, 季数 %d, 错误信息 %v", cid, mediaFile.SeasonNumber, cerr)
			} else {
				if season.AirDate != "" {
					syear := helpers.ParseYearFromDate(season.AirDate)
					if syear != 0 {
						// 检查年份是否相同
						if syear != year {
							helpers.AppLogger.Errorf("tmdb id %d, 季数 %d, 季的年份 %d 与文件中的年份 %d 不匹配", cid, mediaFile.SeasonNumber, syear, year)
							return nil, fmt.Errorf("tmdb id %d, 季数 %d, 季的年份 %d 与文件中的年份 %d 不匹配", cid, mediaFile.SeasonNumber, syear, year)
						}
					}
				}
				return &helpers.MediaInfo{
					TmdbId: cid,
					Name:   cname,
					Year:   cyear,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("名称 %s, 年份 %d, 无法从tmdb查询到匹配的电视剧信息", name, year)
}
