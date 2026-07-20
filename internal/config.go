package internal

import (
	"encoding/json"
	"os"
)

type Config struct {
	SiteName           string `json:"site_name"`
	SiteURL            string `json:"site_url"`
	SiteDescription    string `json:"site_description"`
	BuildDir           string `json:"build_dir"`
	MDDir              string `json:"md_dir"`
	AssetsDir          string `json:"assets_dir"`
	CopyrightName      string `json:"copyright_name"`
	FeaturedPostsCount int    `json:"featured_posts_count"`
	RSSPostsCount      int    `json:"rss_posts_count"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
