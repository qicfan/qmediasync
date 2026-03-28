package scrape

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/tmdb"
	"context"
	"errors"
	"fmt"
	"strings"
)

// 从tmdb刮削元数据
type TmdbMovieImpl struct {
	TmdbBase
}

func NewTmdbMovieImpl(scrapePath *models.ScrapePath, ctx context.Context) *TmdbMovieImpl {
	return &TmdbMovieImpl{
		TmdbBase: TmdbBase{
			scrapePath: scrapePath,
			ctx:        ctx,
			Client:     models.GlobalScrapeSettings.GetTmdbClient(),
		},
	}
}

// 检查电影是否存在
func (t *TmdbMovieImpl) CheckByNameAndYear(name string, year int, switchYear bool) (string, int64, int, error) {
	// 查询电影详情
	movieDetail, err := t.Client.SearchMovie(name, year, models.GlobalScrapeSettings.GetTmdbLanguage(), true, !switchYear)
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电影详情失败, 下次重试, 失败原因: %v", err)
		return "", 0, 0, err
	}
	if movieDetail != nil && movieDetail.TotalResults > 0 && len(movieDetail.Results) > 0 {
		if movieDetail.TotalResults > 1 {
			// 如果第一个数据的title等于name则认为是正确的
			if strings.EqualFold(movieDetail.Results[0].Title, name) || strings.EqualFold(movieDetail.Results[0].OriginalTitle, name) {
				return movieDetail.Results[0].Title, movieDetail.Results[0].ID, helpers.ParseYearFromDate(movieDetail.Results[0].ReleaseDate), nil
			}
			helpers.AppLogger.Infof("tmdb查询到多部电影，第一个电影标题 %s => %s， 年份 %d => %d", movieDetail.Results[0].Title, name, helpers.ParseYearFromDate(movieDetail.Results[0].ReleaseDate), year)
			errorStr := fmt.Sprintf("通过名称 %s 年份 %d 在TMDB无法查询到多部电影，需要手工重新识别输入确定的tmdb id", name, year)
			helpers.AppLogger.Error(errorStr)
			return "", 0, 0, errors.New("多条记录")
		} else {
			return movieDetail.Results[0].Title, movieDetail.Results[0].ID, helpers.ParseYearFromDate(movieDetail.Results[0].ReleaseDate), nil
		}
	} else if movieDetail != nil && movieDetail.TotalResults == 0 {
		if switchYear {
			// 换一个年份字段
			return t.CheckByNameAndYear(name, year, !switchYear)
		}
		return "", 0, 0, errors.New("tmdb没有数据")
	} else {
		return "", 0, 0, errors.New("tmdb没有数据")
	}
}

// 去tmdb查询是否存在
func (t *TmdbMovieImpl) CheckByTmdbId(tmdbId int64) (string, int, error) {
	// 查询电影详情
	movieDetail, err := t.Client.GetMovieDetail(tmdbId, models.GlobalScrapeSettings.GetTmdbLanguage())
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电影详情失败, 下次重试, 失败原因: %v", err)
		return "", 0, err
	}
	return movieDetail.Title, helpers.ParseYearFromDate(movieDetail.ReleaseDate), nil
}

// 检查季是否存在
func (t *TmdbMovieImpl) CheckSeasonByTmdbId(tmdbId int64, seasonNumber int) (*tmdb.SeasonDetail, error) {
	return nil, nil
}
