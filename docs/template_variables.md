# 模板引擎变量说明

本文档说明新版模板引擎（基于 pongo2/Jinja2 语法）支持的所有变量。

## 模板语法

### 变量语法
- **旧语法**（兼容模式）：`{title}`、`{year}`
- **新语法**（推荐）：`{{title}}`、`{{year}}`

### 条件语法
```jinja2
{% if year %}({{year}}){% endif %}
{% if actors %} - {{actors}}{% endif %}
```

### 语法自动检测
系统会自动检测模板语法：
- 包含 `{{` 和 `}}` 的模板会被识别为新语法
- 包含 `{%` 的模板会被识别为新语法
- 否则使用旧语法

## 变量列表

### 通用变量（电影和电视剧）

| 变量名 | 类型 | 说明 | 示例值 |
|--------|------|------|--------|
| `title` | string | 中文标题 | 星际穿越 |
| `year` | int | 上映年份 | 2014 |
| `tmdbid` | int | TMDB ID | 157336 |
| `videoFormat` | string | 视频分辨率 | 1080p, 2160p, 720p |
| `edition` | string | 分辨率等级 | FHD, UHD, HD |
| `fileExt` | string | 文件扩展名 | .mkv, .mp4, .avi |
| `original_title` | string | 原始标题（英文） | Interstellar |
| `original_language` | string | 原始语言 | en, zh, ja |
| `imdbid` | string | IMDB ID | tt0816692 |
| `runtime` | int | 运行时间（分钟） | 169 |
| `overview` | string | 剧情简介 | 一群探险家利用虫洞进行星际旅行 |
| `vote_average` | float | 评分 | 8.600000 |
| `videoCodec` | string | 视频编码 | h264, hevc, vp9 |
| `audioCodec` | string | 音频编码 | aac, dts, ac3 |
| `actors` | string | 演员列表（智能处理） | 马修·麦康纳, 安妮·海瑟薇 |
| `num` | string | 编号 | ABCD-1234 |

#### actors 变量特殊处理
- **无演员**：返回空字符串 `""`
- **1位演员**：返回演员姓名，如 `马修·麦康纳`
- **2位演员**：返回两位演员姓名，用逗号分隔，如 `马修·麦康纳, 安妮·海瑟薇`
- **3位及以上**：返回 `多人演员`

### 电视剧专用变量

| 变量名 | 类型 | 说明 | 示例值 |
|--------|------|------|--------|
| `season` | int | 季号 | 2 |
| `episode` | int | 集号 | 8 |
| `season_episode` | string | 季集格式化字符串 | S02E08 |
| `episode_title` | string | 集标题 | 猎魔人之战 |
| `season_year` | int | 季年份 | 2023 |

## 模板示例

### 基础电影模板
```jinja2
{{title}} ({{year}})
```
输出：`星际穿越 (2014)`

### 完整电影模板
```jinja2
{{title}} ({{year}}) [{{videoFormat}}] {{edition}} - {{actors}}
```
输出：`星际穿越 (2014) [1080p] FHD - 马修·麦康纳, 安妮·海瑟薇`

### 带条件的电影模板
```jinja2
{{title}}{% if year %} ({{year}}){% endif %}{% if actors %} - {{actors}}{% endif %}
```
- 有年份和演员：`星际穿越 (2014) - 马修·麦康纳, 安妮·海瑟薇`
- 无年份无演员：`小众电影`

### 基础电视剧模板
```jinja2
{{title}} {{season_episode}}
```
输出：`猎魔人 S02E08`

### 完整电视剧模板
```jinja2
{{title}} {{season_episode}} - {{episode_title}} [{{videoFormat}}]
```
输出：`猎魔人 S02E08 - 猎魔人之战 [1080p]`

### MoviePilot 兼容模板
```jinja2
{{title}}{% if year %} ({{year}}){% endif %}{% if videoFormat %} [{{videoFormat}}]{% endif %}{% if actors %} - {{actors}}{% endif %}
```

## 旧语法兼容

为了向后兼容，旧语法仍然支持：

| 旧语法变量 | 等效新语法变量 |
|------------|----------------|
| `{title}` | `{{title}}` |
| `{year}` | `{{year}}` |
| `{resolution}` | `{{videoFormat}}` |
| `{tmdb_id}` | `{{tmdbid}}` |
| `{actors}` | `{{actors}}` |

旧语法示例：
```
{title} ({year}) [{resolution}] {actors}
```
输出：`星际穿越 (2014) [1080p] 马修·麦康纳, 安妮·海瑟薇`

## 注意事项

1. **语法自动检测**：系统会自动检测使用新语法还是旧语法，无需手动切换
2. **空值处理**：新语法支持条件判断，可以优雅地处理空值
3. **浮点数精度**：`vote_average` 会输出完整精度（如 `8.600000`），如需格式化可使用 Jinja2 过滤器
4. **演员列表**：`actors` 变量会根据演员数量智能处理，3位以上显示"多人演员"
5. **默认模板**：如果模板为空，默认使用 `{title} ({year})`

## 技术实现

- **模板引擎**：[pongo2](https://github.com/flosch/pongo2) (Jinja2 兼容)
- **语法标准**：Jinja2 模板语法
- **兼容性**：完全兼容旧语法，自动检测并切换引擎
