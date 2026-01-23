package helpers

import (
	"encoding/xml"
	"io"
	"os"
)

type TVShowEpisode struct {
	XMLName       xml.Name   `xml:"episodedetails"`
	Outline       string     `xml:"outline,omitempty"`
	Plot          string     `xml:"plot,omitempty"`
	Tagline       string     `xml:"tagline,omitempty"`
	Title         string     `xml:"title,omitempty"`
	OriginalTitle string     `xml:"originaltitle,omitempty"`
	SortTitle     string     `xml:"sorttitle,omitempty"`
	Premiered     string     `xml:"premiered,omitempty"`
	Releasedate   string     `xml:"releasedate,omitempty"`
	Year          int        `xml:"year,omitempty"`
	SeasonNumber  int        `xml:"seasonnumber,omitempty"`
	EpisodeNumber int        `xml:"episodenumber,omitempty"`
	Season        int        `xml:"season,omitempty"`
	Episode       int        `xml:"episode,omitempty"`
	DateAdded     string     `xml:"dateadded,omitempty"`
	Actor         []Actor    `xml:"actor,omitempty"`
	Director      []Director `xml:"director,omitempty"`
	// 评分信息
	Rating float64 `xml:"rating,omitempty"`
	Votes  int     `xml:"votes,omitempty"`
}

func ReadEpisodeNfo(r io.Reader) (*TVShowEpisode, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	m := TVShowEpisode{}
	err = xml.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func WriteEpisodeNfo(m *TVShowEpisode, path string) error {
	xmlHeader := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n")
	data, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	content := append(xmlHeader, data...)
	return os.WriteFile(path, content, 0766)
}
