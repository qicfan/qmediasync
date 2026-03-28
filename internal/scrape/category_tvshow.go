package scrape

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"slices"
)

// 分类电视剧文件
type CategoryTvShowImpl struct {
	scrapePath *models.ScrapePath
}

func NewCategoryTvShowImpl(scrapePath *models.ScrapePath) *CategoryTvShowImpl {
	return &CategoryTvShowImpl{
		scrapePath: scrapePath,
	}
}

// 分类电视剧文件
func (ct *CategoryTvShowImpl) DoCategory(mediaFile *models.ScrapeMediaFile) (string, *models.ScrapePathCategory) {
	if mediaFile.Media == nil {
		return "", nil
	}
	var c *models.TvShowCategory
	genres := mediaFile.Media.Genres
	originalCountry := mediaFile.Media.OriginCountry
	helpers.AppLogger.Infof("计算电视剧的二级分类，影片名称: %s 流派 %+v 和国家 %+v 的二级分类", mediaFile.Media.Name, genres, originalCountry)
	if len(ct.scrapePath.Category.TvShowCategory) == 0 {
		helpers.AppLogger.Warnf("电视剧分类规则为空，跳过分类")
		return "", nil
	}
	defaultCategory := ct.scrapePath.Category.TvShowCategory[0]
	if len(genres) == 0 && len(originalCountry) > 0 {
		helpers.AppLogger.Infof("电视剧只有国家 %+v 要求，没有流派", originalCountry)
		// 只匹配有国家要求的
	countryloop:
		for _, country := range originalCountry {
			for _, tvShowCategory := range ct.scrapePath.Category.TvShowCategory {
				// helpers.AppLogger.Infof("检查电视剧 %s 国家 %+v 是否命中分类 %s 的国家: %+v", media.Name, originalCountry, tvShowCategory.Name, tvShowCategory.CountryArray)
				if slices.Contains(tvShowCategory.CountryArray, country) {
					helpers.AppLogger.Infof("分类 %s 的国家 %+v 命中要求的国家 %s", tvShowCategory.Name, tvShowCategory.CountryArray, country)
					c = tvShowCategory
					break countryloop
				} else {
					helpers.AppLogger.Infof("分类 %s 的国家 %+v 未命中要求的国家 %s", tvShowCategory.Name, tvShowCategory.CountryArray, country)
				}
			}
		}
		if c == nil {
			// 没有语言命中，返回默认分类
			c = defaultCategory
		}
	} else if len(genres) > 0 && len(originalCountry) == 0 {
		helpers.AppLogger.Infof("只有流派 %+v 要求，不匹配国家", genres)
		// 只匹配有流派要求的
	gennerloop:
		for _, genre := range genres {
			for _, tvShowCategory := range ct.scrapePath.Category.TvShowCategory {
				if slices.Contains(tvShowCategory.GenreIdArray, genre.ID) {
					helpers.AppLogger.Debugf("流派ID %d 命中分类 %s 的流派ID: %+v", genre.ID, tvShowCategory.Name, tvShowCategory.GenreIdArray)
					c = tvShowCategory
					break gennerloop
				} else {
					helpers.AppLogger.Infof("流派ID %d 未命中分类 %s 的流派ID: %+v", genre.ID, tvShowCategory.Name, tvShowCategory.GenreIdArray)
				}
			}
		}
		if c == nil {
			// 没有流派命中，返回默认分类
			c = defaultCategory
		}
	} else {
		// 匹配同时有流派和国家要求的
		// 取fc和fl的交集
		fC := make([]*models.TvShowCategory, 0)
		fCA := make([]*models.TvShowCategory, 0)
		fL := make([]*models.TvShowCategory, 0)
		fLA := make([]*models.TvShowCategory, 0)
		// 检查流派ID是否命中
		for _, genre := range genres {
			for _, tvShowCategory := range ct.scrapePath.Category.TvShowCategory {
				if tvShowCategory.GenreIdArray == nil || (tvShowCategory.GenreIdArray != nil && len(tvShowCategory.GenreIdArray) == 0) {
					helpers.AppLogger.Debugf("分类 %s 没有流派要求，直接命中", tvShowCategory.Name)
					fCA = append(fCA, tvShowCategory)
				}
				if slices.Contains(tvShowCategory.GenreIdArray, genre.ID) {
					helpers.AppLogger.Debugf("流派ID %d 命中分类 %s 的流派ID: %+v", genre.ID, tvShowCategory.Name, tvShowCategory.GenreIdArray)
					fC = append(fC, tvShowCategory)
				}
			}
		}

		// 检查国家是否命中
		for _, country := range originalCountry {
			for _, tvShowCategory := range ct.scrapePath.Category.TvShowCategory {
				if tvShowCategory.CountryArray == nil || (tvShowCategory.CountryArray != nil && len(tvShowCategory.CountryArray) == 0) {
					// 全部国家直接命中
					helpers.AppLogger.Debugf("分类 %s 没有国家要求，直接命中", tvShowCategory.Name)
					fLA = append(fLA, tvShowCategory)
					continue
				}
				if slices.Contains(tvShowCategory.CountryArray, country) {
					helpers.AppLogger.Debugf("分类 %s 的国家 %+v 命中要求的国家 %s", tvShowCategory.Name, tvShowCategory.CountryArray, country)
					fL = append(fL, tvShowCategory)
					continue
				}
			}
		}
		helpers.AppLogger.Infof("有 %d 个没有流派要求的分类命中", len(fCA))
		helpers.AppLogger.Infof("有 %d 个有流派要求的分类命中", len(fC))
		helpers.AppLogger.Infof("有 %d 个没有国家要求的分类命中", len(fLA))
		helpers.AppLogger.Infof("有 %d 个有国家要求的分类命中", len(fL))
		if len(fC) > 0 && len(fL) > 0 {
			for _, tempC := range fC {
				if slices.Contains(fL, tempC) {
					c = tempC
					helpers.AppLogger.Infof("取流派ID和国家均命中的分类 %d=>%s", c.ID, c.Name)
					break
				}
			}
			// 如果没有交集，优先匹配流派
			if c == nil {
				c = fC[0]
				helpers.AppLogger.Infof("取流派ID命中的分类 %d=>%s", c.ID, c.Name)
			}
		}
		// 如果有精确的国家命中，没有精确的流派命中
		if len(fL) > 0 && len(fC) == 0 {
			for _, tempL := range fL {
				if slices.Contains(fL, tempL) {
					c = tempL
					helpers.AppLogger.Infof("取国家命中的分类 %d=>%s", c.ID, c.Name)
					break
				}
			}
		}
		// 如果有精确的流派命中，没有精确的语言命中
		if len(fC) > 0 && len(fL) == 0 {
			for _, tempC := range fC {
				if slices.Contains(fC, tempC) {
					c = tempC
					helpers.AppLogger.Infof("取流派ID命中的分类 %d=>%s", c.ID, c.Name)
					break
				}
			}
		}

		if c == nil {
			// 没有交集，优先取有流派要求的分类
			if len(fCA) > 0 {
				c = fCA[0]
				helpers.AppLogger.Infof("取没有流派要求的分类 %d=>%s", c.ID, c.Name)
			} else if len(fLA) > 0 {
				c = fLA[0]
				helpers.AppLogger.Infof("取没有国家要求的分类 %d=>%s", c.ID, c.Name)
			} else {
				// 没有交集，返回默认分类
				c = defaultCategory
				helpers.AppLogger.Infof("流派ID和国家均未命中分类, 自动选择默认分类 %d=>%s", c.ID, c.Name)
			}
		}
	}
	// 取c对应的ScrapePathCategory
	for _, spC := range ct.scrapePath.Category.PathCategory {
		if spC.CategoryId == c.ID {
			return c.Name, spC
		}
	}
	return "", nil
}
