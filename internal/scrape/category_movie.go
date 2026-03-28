package scrape

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"slices"
)

// 分类电影文件
type CategoryMovieImpl struct {
	scrapePath *models.ScrapePath
}

func NewCategoryMovieImpl(scrapePath *models.ScrapePath) *CategoryMovieImpl {
	return &CategoryMovieImpl{
		scrapePath: scrapePath,
	}
}

// 分类电影文件
func (cm *CategoryMovieImpl) DoCategory(mediaFile *models.ScrapeMediaFile) (string, *models.ScrapePathCategory) {
	if mediaFile.Media == nil {
		return "", nil
	}
	var c *models.MovieCategory
	genres := mediaFile.Media.Genres
	originalLanguage := mediaFile.Media.OriginalLanguage
	helpers.AppLogger.Infof("计算电影的二级分类，影片名称: %s 流派 %+v 和语言 %s", mediaFile.Media.Name, genres, originalLanguage)
	if len(cm.scrapePath.Category.MovieCategory) == 0 {
		helpers.AppLogger.Warnf("电影分类规则为空，跳过分类")
		return "", nil
	}
	defaultCategory := cm.scrapePath.Category.MovieCategory[0]
	if len(genres) == 0 && originalLanguage != "" {
		helpers.AppLogger.Infof("只有语言 %s 要求，不匹配流派", originalLanguage)
		// 只匹配有语言要求的
		for _, movieCategory := range cm.scrapePath.Category.MovieCategory {
			// helpers.AppLogger.Infof("检查电影 %s 语言 %s 是否命中分类 %s 的语言: %+v", media.Name, originalLanguage, movieCategory.Name, movieCategory.LanguageArray)
			if slices.Contains(movieCategory.LanguageArray, originalLanguage) {
				helpers.AppLogger.Infof("分类 %s 的语言 %+v 命中要求的语言 %s", movieCategory.Name, movieCategory.LanguageArray, originalLanguage)
				c = movieCategory
				break
			} else {
				helpers.AppLogger.Infof("分类 %s 的语言 %+v 未命中要求的语言 %s", movieCategory.Name, movieCategory.LanguageArray, originalLanguage)
			}
		}
		if c == nil {
			// 没有语言命中，返回默认分类
			c = defaultCategory
		}
	} else if len(genres) > 0 && originalLanguage == "" {
		helpers.AppLogger.Infof("只有流派 %+v 要求，不匹配语言", genres)
		// 只匹配有流派要求的
	gennerloop:
		for _, genre := range genres {
			for _, movieCategory := range cm.scrapePath.Category.MovieCategory {
				if slices.Contains(movieCategory.GenreIdArray, genre.ID) {
					helpers.AppLogger.Debugf("流派ID %d 命中分类 %s 的流派ID: %+v", genre.ID, movieCategory.Name, movieCategory.GenreIdArray)
					c = movieCategory
					break gennerloop
				} else {
					helpers.AppLogger.Infof("流派ID %d 未命中分类 %s 的流派ID: %+v", genre.ID, movieCategory.Name, movieCategory.GenreIdArray)
				}
			}
		}
		if c == nil {
			// 没有流派命中，返回默认分类
			c = defaultCategory
		}
	} else {
		// 匹配同时有流派和语言要求的
		// 取fc和fl的交集
		fC := make([]*models.MovieCategory, 0)
		fCA := make([]*models.MovieCategory, 0)
		fL := make([]*models.MovieCategory, 0)
		fLA := make([]*models.MovieCategory, 0)
		// helpers.AppLogger.Infof("电影 %s 流派ID: %v, 语言: %s", media.Name, media.TmdbInfo.MovieDetail.Genres, media.TmdbInfo.MovieDetail.OriginalLanguage)
		// 检查流派ID是否命中
		for _, genre := range genres {
			for _, movieCategory := range cm.scrapePath.Category.MovieCategory {
				if movieCategory.GenreIdArray == nil || (movieCategory.GenreIdArray != nil && len(movieCategory.GenreIdArray) == 0) {
					// helpers.AppLogger.Debugf("分类 %s 没有流派要求，直接命中", movieCategory.Name)
					fCA = append(fCA, movieCategory)
				}
				if slices.Contains(movieCategory.GenreIdArray, genre.ID) {
					// helpers.AppLogger.Debugf("流派ID %d 命中分类 %s 的流派ID: %+v", genre.ID, movieCategory.Name, movieCategory.GenreIdArray)
					fC = append(fC, movieCategory)
				}
			}
		}

		// 检查是否有精确的语言命中
		for _, movieCategory := range cm.scrapePath.Category.MovieCategory {
			if movieCategory.LanguageArray == nil || (movieCategory.LanguageArray != nil && len(movieCategory.LanguageArray) == 0) {
				helpers.AppLogger.Debugf("分类 %s 没有语言要求，直接命中", movieCategory.Name)
				fLA = append(fLA, movieCategory)
			}
			// helpers.AppLogger.Infof("检查电影 %s 语言 %s 是否命中分类 %s 的语言: %+v", media.Name, originalLanguage, movieCategory.Name, movieCategory.LanguageArray)
			if slices.Contains(movieCategory.LanguageArray, originalLanguage) {
				helpers.AppLogger.Debugf("分类 %s 的语言 %+v 命中要求的语言 %s", movieCategory.Name, movieCategory.LanguageArray, originalLanguage)
				fL = append(fL, movieCategory)
			}
		}
		// helpers.AppLogger.Infof("有 %d 个没有流派要求的分类命中", len(fCA))
		// helpers.AppLogger.Infof("有 %d 个有流派要求的分类命中", len(fC))
		// helpers.AppLogger.Infof("有 %d 个没有语言要求的分类命中", len(fLA))
		// helpers.AppLogger.Infof("有 %d 个有语言要求的分类命中", len(fL))
		// 从既有流派要求又有语言要求的记录中取交集
		if len(fC) > 0 && len(fL) > 0 {
			for _, tempC := range fC {
				if slices.Contains(fL, tempC) {
					c = tempC
					helpers.AppLogger.Infof("取流派ID和语言均命中的分类 %d=>%s", c.ID, c.Name)
					break
				}
			}
			// 如果没有交集，优先匹配流派
			if c == nil {
				c = fC[0]
				helpers.AppLogger.Infof("取流派ID命中的分类 %d=>%s", c.ID, c.Name)
			}
		}
		// 如果有精确的语言命中，没有精确的流派命中
		if len(fL) > 0 && len(fC) == 0 {
			for _, tempL := range fL {
				if slices.Contains(fL, tempL) {
					c = tempL
					// helpers.AppLogger.Infof("取语言命中的分类 %d=>%s", c.ID, c.Name)
					break
				}
			}
		}
		// 如果有精确的流派命中，没有精确的语言命中
		if len(fC) > 0 && len(fL) == 0 {
			for _, tempC := range fC {
				if slices.Contains(fC, tempC) {
					c = tempC
					// helpers.AppLogger.Infof("取流派ID命中的分类 %d=>%s", c.ID, c.Name)
					break
				}
			}
		}
		if c == nil {
			// 没有交集，优先取有流派要求的分类
			if len(fCA) > 0 {
				c = fCA[0]
				// helpers.AppLogger.Infof("取没有流派要求的分类 %d=>%s", c.ID, c.Name)
			} else if len(fLA) > 0 {
				c = fLA[0]
				// helpers.AppLogger.Infof("取没有语言要求的分类 %d=>%s", c.ID, c.Name)
			} else {
				// 没有交集，返回默认分类
				c = defaultCategory
				// helpers.AppLogger.Infof("流派ID和语言均未命中分类, 自动选择默认分类 %d=>%s", c.ID, c.Name)
			}
		}
	}
	// 取c对应的ScrapePathCategory
	for _, spC := range cm.scrapePath.Category.PathCategory {
		if spC.CategoryId == c.ID {
			return c.Name, spC
		}
	}
	return "", nil
}
