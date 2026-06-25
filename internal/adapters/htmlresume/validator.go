package htmlresume

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"golang.org/x/net/html"
)

type ValidatedHTML struct {
	HTML      string
	HTMLHash  string
	StyleHash string
}

type Validator struct{}

var whitespacePattern = regexp.MustCompile(`\s+`)
var cssClassPattern = regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9_-]*)`)

func (Validator) Validate(
	resume domain.Resume,
	template ports.ResumeHTMLTemplate,
	contents string,
) (ValidatedHTML, error) {
	resume = domain.NormalizeResume(resume)
	contents = strings.TrimSpace(contents)
	if contents == "" {
		return ValidatedHTML{}, errors.New("resume HTML is empty")
	}
	if strings.HasPrefix(contents, "```") || strings.HasSuffix(contents, "```") {
		return ValidatedHTML{}, errors.New("resume HTML contains Markdown fences")
	}
	style, err := extractStyle(contents)
	if err != nil {
		return ValidatedHTML{}, err
	}
	styleDigest := sha256.Sum256([]byte(style))
	styleHash := hex.EncodeToString(styleDigest[:])
	if styleHash != template.StyleHash {
		return ValidatedHTML{}, errors.New(
			"resume HTML style does not match template",
		)
	}

	root, err := html.Parse(strings.NewReader(contents))
	if err != nil {
		return ValidatedHTML{}, fmt.Errorf("parse resume HTML: %w", err)
	}
	if err := validateTree(root, allowedClasses(template.HTML)); err != nil {
		return ValidatedHTML{}, err
	}
	if err := validateResumeBindings(root, resume); err != nil {
		return ValidatedHTML{}, err
	}
	if strings.Contains(contents, "{{") || strings.Contains(contents, "}}") {
		return ValidatedHTML{}, errors.New(
			"resume HTML still contains template placeholders",
		)
	}

	htmlDigest := sha256.Sum256([]byte(contents))
	return ValidatedHTML{
		HTML:      contents,
		HTMLHash:  hex.EncodeToString(htmlDigest[:]),
		StyleHash: styleHash,
	}, nil
}

func validateTree(node *html.Node, classes map[string]struct{}) error {
	if node.Type == html.ElementNode {
		if err := validateElement(node, classes); err != nil {
			return err
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if err := validateTree(child, classes); err != nil {
			return err
		}
	}
	return nil
}

func validateElement(node *html.Node, classes map[string]struct{}) error {
	switch node.Data {
	case "script", "iframe", "object", "embed", "form", "input",
		"textarea", "select", "button", "link":
		return fmt.Errorf("resume HTML tag <%s> is not allowed", node.Data)
	}
	for _, attr := range node.Attr {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		value := strings.TrimSpace(attr.Val)
		if strings.HasPrefix(key, "on") {
			return fmt.Errorf("resume HTML event attribute %q is not allowed", key)
		}
		if key == "style" {
			return errors.New("resume HTML inline style is not allowed")
		}
		if key == "class" {
			for _, className := range strings.Fields(value) {
				if _, exists := classes[className]; !exists {
					return fmt.Errorf(
						"resume HTML class %q is not in template",
						className,
					)
				}
			}
		}
		if key == "href" {
			if !isAllowedHref(value) {
				return fmt.Errorf("resume HTML href %q is not allowed", value)
			}
		}
		if key == "src" {
			return errors.New("resume HTML src resources are not allowed")
		}
	}
	return nil
}

func allowedClasses(templateHTML string) map[string]struct{} {
	root, err := html.Parse(strings.NewReader(templateHTML))
	if err != nil {
		return map[string]struct{}{}
	}
	classes := make(map[string]struct{})
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for _, attr := range node.Attr {
				if attr.Key != "class" {
					continue
				}
				for _, className := range strings.Fields(attr.Val) {
					classes[className] = struct{}{}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if style, err := extractStyle(templateHTML); err == nil {
		for _, match := range cssClassPattern.FindAllStringSubmatch(style, -1) {
			classes[match[1]] = struct{}{}
		}
	}
	return classes
}

func isAllowedHref(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" {
		return strings.HasPrefix(value, "#")
	}
	switch parsed.Scheme {
	case "https", "mailto", "tel":
		return true
	default:
		return false
	}
}

func validateResumeBindings(root *html.Node, resume domain.Resume) error {
	expectedIDs, expectedFields := expectedBindings(resume)
	actualIDs := make(map[string]int)
	actualFields := make(map[string][]string)
	var walk func(*html.Node, string)
	walk = func(node *html.Node, owner string) {
		if node.Type == html.ElementNode {
			if id, ok := attr(node, "data-autocv-id"); ok {
				owner = id
				actualIDs[id]++
			}
			if field, ok := attr(node, "data-autocv-field"); ok {
				key := field
				if owner != "" {
					key = owner + ":" + field
				}
				actualFields[key] = append(
					actualFields[key],
					collapsedText(node),
				)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, owner)
		}
	}
	walk(root, "")

	for id := range expectedIDs {
		if actualIDs[id] != 1 {
			return fmt.Errorf(
				"resume HTML id %q appears %d times, want 1",
				id,
				actualIDs[id],
			)
		}
	}
	for key, want := range expectedFields {
		values := actualFields[key]
		if len(values) != 1 {
			return fmt.Errorf(
				"resume HTML binding %q appears %d times, want 1",
				key,
				len(values),
			)
		}
		if normalizeText(values[0]) != normalizeText(want) {
			return fmt.Errorf(
				"resume HTML binding %q changed text",
				key,
			)
		}
	}
	return nil
}

func expectedBindings(resume domain.Resume) (map[string]struct{}, map[string]string) {
	ids := make(map[string]struct{})
	fields := make(map[string]string)
	if strings.TrimSpace(resume.Header.Name) != "" {
		fields["header.name"] = resume.Header.Name
	}
	fields["header.target_role"] = resume.Header.TargetRole
	for _, section := range resume.Sections {
		ids[section.ID] = struct{}{}
		fields[section.ID+":section.title"] = section.Title
		for _, item := range section.Items {
			ids[item.ID] = struct{}{}
			fields[item.ID+":item.content"] = item.Content
		}
	}
	return ids, fields
}

func attr(node *html.Node, key string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

func collapsedText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteString(" ")
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return normalizeText(builder.String())
}

func normalizeText(value string) string {
	return strings.TrimSpace(whitespacePattern.ReplaceAllString(value, " "))
}
