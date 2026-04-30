package cloud

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

func (s *CloudServer) handleGetAvatar(c *gin.Context) {
	username := c.Param("username")
	svg := generateAvatar(username)
	c.Data(http.StatusOK, "image/svg+xml", []byte(svg))
}

func generateAvatar(username string) string {
	initial := strings.ToUpper(firstChar(username))
	bg := usernameColor(username)

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
  <rect width="64" height="64" rx="8" fill="%s"/>
  <text x="32" y="32" dy=".35em" text-anchor="middle" fill="#fff" font-family="sans-serif" font-size="28" font-weight="600">%s</text>
</svg>`, bg, initial)
}

func firstChar(s string) string {
	if s == "" {
		return "?"
	}
	r, _ := utf8.DecodeRuneInString(s)
	return string(r)
}

func usernameColor(username string) string {
	hash := md5.Sum([]byte(username))
	h := int(hash[0]) % 360
	return fmt.Sprintf("hsl(%d, 60%%, 45%%)", h)
}
