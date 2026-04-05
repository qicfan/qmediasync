package covergen

type CoverStyle string

const (
	CoverStyleSingle CoverStyle = "single"
	CoverStyleGrid   CoverStyle = "grid"
)

type CoverSort string

const (
	CoverSortRandom       CoverSort = "random"
	CoverSortDateCreated  CoverSort = "date_created"
	CoverSortPremiereDate CoverSort = "premiere_date"
)

type CoverGenRequest struct {
	LibraryIDs []string   `json:"library_ids"`
	Style      CoverStyle `json:"style"`
	ItemIDs    []string   `json:"item_ids"`
}

type CoverGenResult struct {
	LibraryID   string `json:"library_id"`
	LibraryName string `json:"library_name"`
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	DurationMs  int64  `json:"duration_ms"`
}

type CoverGenSummary struct {
	Total   int              `json:"total"`
	Success int              `json:"success"`
	Failed  int              `json:"failed"`
	Results []CoverGenResult `json:"results"`
}

type CoverGenConfig struct {
	ZhFontSize   float64   `json:"zh_font_size"`
	EnFontSize   float64   `json:"en_font_size"`
	TitleSpacing float64   `json:"title_spacing"`
	BlurSize     int       `json:"blur_size"`
	ColorRatio   float64   `json:"color_ratio"`
	Resolution   string    `json:"resolution"`
	UsePrimary   bool      `json:"use_primary"`
	MultiBlur    bool      `json:"multi_blur"`
	TitleConfig  string    `json:"title_config"`
	SortBy       CoverSort `json:"sort_by"`
}

type CoverGenStatus struct {
	IsRunning       bool            `json:"is_running"`
	LastRunTime     string          `json:"last_run_time"`
	LastRunResults  []CoverGenResult `json:"last_run_results"`
}

type FontStatus struct {
	Available bool   `json:"available"`
	Source    string `json:"source"`
	Path      string `json:"path"`
}

type FontsStatus struct {
	ZhFont FontStatus `json:"zh_font"`
	EnFont FontStatus `json:"en_font"`
}

type PreviewRequest struct {
	LibraryID string   `json:"library_id"`
	Style     CoverStyle `json:"style"`
	Title     []string `json:"title"`
}
