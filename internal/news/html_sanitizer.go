package news

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	hexColorPattern  = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)
	fontSizePattern  = regexp.MustCompile(`^(14|16|18|20|24|28|32|36|40)px$`)
	youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,}$`)
	vimeoIDPattern   = regexp.MustCompile(`^[0-9]+$`)
	spotifyIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)
)

var allowedFontFamilies = map[string]bool{
	"var(--font-libre-caslon-text)": true,
	"var(--font-plus-jakarta-sans)": true,
	"georgia, serif":                true,
	"arial, sans-serif":             true,
	`"times new roman", serif`:      true,
	`'times new roman', serif`:      true,
	`"courier new", monospace`:      true,
	`'courier new', monospace`:      true,
}

var allowedElements = map[string]bool{
	"a": true, "audio": true, "b": true, "blockquote": true, "br": true,
	"code": true, "em": true, "h2": true, "h3": true, "hr": true,
	"i": true, "iframe": true, "img": true, "li": true, "ol": true,
	"p": true, "pre": true, "s": true, "span": true, "strike": true,
	"strong": true, "u": true, "ul": true, "video": true,
}

var blockedElements = map[string]bool{
	"applet": true, "canvas": true, "embed": true, "form": true, "input": true,
	"math": true, "object": true, "script": true, "style": true, "svg": true,
	"template": true,
}

func sanitizeNewsHTML(value string) (string, error) {
	context := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(value), context)
	if err != nil {
		return "", fmt.Errorf("parse body HTML: %w", err)
	}

	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, node := range nodes {
		sanitizeNode(root, node)
	}

	var output bytes.Buffer
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&output, child); err != nil {
			return "", fmt.Errorf("render body HTML: %w", err)
		}
	}
	return strings.TrimSpace(output.String()), nil
}

func sanitizeNode(parent *html.Node, source *html.Node) {
	switch source.Type {
	case html.TextNode:
		parent.AppendChild(&html.Node{Type: html.TextNode, Data: source.Data})
		return
	case html.ElementNode:
		tag := strings.ToLower(source.Data)
		if blockedElements[tag] {
			return
		}
		if !allowedElements[tag] {
			for child := source.FirstChild; child != nil; child = child.NextSibling {
				sanitizeNode(parent, child)
			}
			return
		}

		target, keepChildren := sanitizeElement(source, tag)
		if target == nil {
			if keepChildren {
				for child := source.FirstChild; child != nil; child = child.NextSibling {
					sanitizeNode(parent, child)
				}
			}
			return
		}

		parent.AppendChild(target)
		if tag == "iframe" || tag == "img" || tag == "audio" || tag == "video" ||
			tag == "br" || tag == "hr" {
			return
		}
		for child := source.FirstChild; child != nil; child = child.NextSibling {
			sanitizeNode(target, child)
		}
	}
}

func sanitizeElement(source *html.Node, tag string) (*html.Node, bool) {
	target := &html.Node{Type: html.ElementNode, Data: tag, DataAtom: atom.Lookup([]byte(tag))}
	attributes := attributesByName(source.Attr)

	switch tag {
	case "a":
		href, ok := sanitizeURL(attributes["href"], true)
		if !ok {
			return nil, true
		}
		appendAttribute(target, "href", href)
		if attributes["target"] == "_blank" {
			appendAttribute(target, "target", "_blank")
			appendAttribute(target, "rel", "noopener noreferrer")
		}
	case "span":
		if style := sanitizeStyle(attributes["style"], map[string]bool{
			"color": true, "font-family": true, "font-size": true, "font-weight": true,
		}); style != "" {
			appendAttribute(target, "style", style)
		}
	case "p", "h2", "h3":
		if style := sanitizeStyle(attributes["style"], map[string]bool{"text-align": true}); style != "" {
			appendAttribute(target, "style", style)
		}
	case "img":
		src, ok := sanitizeURL(attributes["src"], false)
		if !ok {
			return nil, false
		}
		appendAttribute(target, "src", src)
		appendOptionalAttribute(target, "alt", attributes["alt"])
		appendOptionalAttribute(target, "title", attributes["title"])
		align := attributes["data-align"]
		if align != "left" && align != "right" && align != "center" && align != "full" {
			align = "full"
		}
		appendAttribute(target, "data-align", align)
		if width, ok := sanitizeWidth(attributes["data-width"]); ok {
			appendAttribute(target, "data-width", width)
			appendAttribute(target, "style", "width: "+width)
		} else if style := sanitizeStyle(attributes["style"], map[string]bool{"width": true}); style != "" {
			appendAttribute(target, "style", style)
		}
	case "audio", "video":
		src, ok := sanitizeURL(attributes["src"], false)
		if !ok {
			return nil, false
		}
		appendAttribute(target, "src", src)
		appendAttribute(target, "controls", "")
		appendAttribute(target, "preload", "metadata")
		if tag == "video" {
			appendAttribute(target, "playsinline", "")
		}
	case "iframe":
		embed, ok := normalizeEmbedURL(attributes["src"])
		if !ok {
			return nil, false
		}
		appendAttribute(target, "src", embed.src)
		appendAttribute(target, "data-provider", embed.provider)
		appendAttribute(target, "title", embed.title)
		appendAttribute(target, "loading", "lazy")
		appendAttribute(target, "referrerpolicy", "strict-origin-when-cross-origin")
		if embed.allow != "" {
			appendAttribute(target, "allow", embed.allow)
		}
		if embed.fullscreen {
			appendAttribute(target, "allowfullscreen", "")
		}
	case "ol":
		if start, err := strconv.Atoi(attributes["start"]); err == nil && start > 1 {
			appendAttribute(target, "start", strconv.Itoa(start))
		}
	}

	return target, false
}

func attributesByName(attributes []html.Attribute) map[string]string {
	result := make(map[string]string, len(attributes))
	for _, attribute := range attributes {
		result[strings.ToLower(attribute.Key)] = strings.TrimSpace(attribute.Val)
	}
	return result
}

func appendAttribute(node *html.Node, key, value string) {
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

func appendOptionalAttribute(node *html.Node, key, value string) {
	if value != "" {
		appendAttribute(node, key, value)
	}
}

func sanitizeURL(raw string, allowContactProtocols bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if strings.HasPrefix(raw, "/api/uploads/") {
		filename := strings.TrimPrefix(raw, "/api/uploads/")
		if storedUploadFilenamePattern.MatchString(filename) {
			return "/api/uploads/" + filename, true
		}
		return "", false
	}
	if allowContactProtocols {
		if strings.HasPrefix(raw, "#") && len(raw) > 1 {
			return raw, true
		}
		if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
			return raw, true
		}
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		if allowContactProtocols && (parsed.Scheme == "mailto" || parsed.Scheme == "tel") {
			return parsed.String(), true
		}
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return parsed.String(), true
}

var storedUploadFilenamePattern = regexp.MustCompile(`^[a-f0-9]{32}\.(jpe?g|png|webp|gif|mp3|wav|ogg|oga|m4a|mp4|webm|ogv)$`)

func sanitizeStyle(raw string, allowed map[string]bool) string {
	var sanitized []string
	for _, declaration := range strings.Split(raw, ";") {
		property, value, found := strings.Cut(declaration, ":")
		if !found {
			continue
		}
		property = strings.ToLower(strings.TrimSpace(property))
		value = strings.TrimSpace(value)
		if !allowed[property] || !validStyleValue(property, value) {
			continue
		}
		sanitized = append(sanitized, property+": "+value)
	}
	return strings.Join(sanitized, "; ")
}

func validStyleValue(property, value string) bool {
	switch property {
	case "color":
		return hexColorPattern.MatchString(value)
	case "font-family":
		return allowedFontFamilies[strings.ToLower(value)]
	case "font-size":
		return fontSizePattern.MatchString(strings.ToLower(value))
	case "font-weight":
		return value == "300" || value == "400" || value == "500" ||
			value == "600" || value == "700" || value == "800"
	case "text-align":
		return value == "left" || value == "center" || value == "right" || value == "justify"
	case "width":
		_, ok := sanitizeWidth(value)
		return ok
	default:
		return false
	}
}

func sanitizeWidth(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasSuffix(value, "%"):
		number, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
		return value, err == nil && number >= 1 && number <= 100
	case strings.HasSuffix(value, "px"):
		number, err := strconv.Atoi(strings.TrimSuffix(value, "px"))
		return value, err == nil && number >= 50 && number <= 1200
	default:
		return "", false
	}
}

type normalizedEmbed struct {
	provider   string
	src        string
	title      string
	allow      string
	fullscreen bool
}

func normalizeEmbedURL(raw string) (normalizedEmbed, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" {
		return normalizedEmbed{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	segments := splitPath(parsed.Path)

	switch host {
	case "www.youtube-nocookie.com", "youtube-nocookie.com", "www.youtube.com", "youtube.com":
		if len(segments) == 2 && segments[0] == "embed" && youtubeIDPattern.MatchString(segments[1]) {
			return normalizedEmbed{
				provider:   "youtube",
				src:        "https://www.youtube-nocookie.com/embed/" + segments[1],
				title:      "YouTube video",
				allow:      "accelerometer; encrypted-media; gyroscope; picture-in-picture; fullscreen",
				fullscreen: true,
			}, true
		}
	case "player.vimeo.com":
		if len(segments) == 2 && segments[0] == "video" && vimeoIDPattern.MatchString(segments[1]) {
			return normalizedEmbed{
				provider:   "vimeo",
				src:        "https://player.vimeo.com/video/" + segments[1],
				title:      "Vimeo video",
				allow:      "fullscreen; picture-in-picture",
				fullscreen: true,
			}, true
		}
	case "open.spotify.com":
		if len(segments) == 3 && segments[0] == "embed" && validSpotifyType(segments[1]) &&
			spotifyIDPattern.MatchString(segments[2]) {
			return normalizedEmbed{
				provider: "spotify",
				src:      "https://open.spotify.com/embed/" + segments[1] + "/" + segments[2],
				title:    "Spotify player",
				allow:    "encrypted-media",
			}, true
		}
	case "w.soundcloud.com":
		if len(segments) == 1 && segments[0] == "player" {
			source, ok := validSoundCloudSource(parsed.Query().Get("url"))
			if !ok {
				return normalizedEmbed{}, false
			}
			query := url.Values{"url": []string{source}}
			return normalizedEmbed{
				provider: "soundcloud",
				src:      "https://w.soundcloud.com/player/?" + query.Encode(),
				title:    "SoundCloud player",
			}, true
		}
	}
	return normalizedEmbed{}, false
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func validSpotifyType(value string) bool {
	return value == "album" || value == "episode" || value == "playlist" ||
		value == "show" || value == "track"
}

func validSoundCloudSource(raw string) (string, bool) {
	source, err := url.Parse(raw)
	if err != nil || source.Scheme != "https" {
		return "", false
	}
	host := strings.ToLower(source.Hostname())
	if host != "soundcloud.com" && host != "www.soundcloud.com" {
		return "", false
	}
	return source.String(), true
}
