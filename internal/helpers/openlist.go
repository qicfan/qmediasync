package helpers

import (
	"fmt"
	"strings"
)

func MakeOpenListUrl(baseUrl, sign, fileId string) string {
	// 去掉BaseUrl末尾的/
	baseUrl = strings.TrimSuffix(baseUrl, "/")
	// 去掉sf.FileId首尾的/
	fileId = strings.Trim(fileId, "/")
	url := fmt.Sprintf("%s/d/%s", baseUrl, fileId)
	if sign != "" {
		url += "?sign=" + sign
	}
	return url
}
