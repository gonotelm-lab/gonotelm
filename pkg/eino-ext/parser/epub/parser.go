package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	stdhtml "html"
	"io"
	"maps"
	"path"
	"strconv"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	xhtml "golang.org/x/net/html"
)

type OutputFormat string

const (
	OutputFormatHTML     OutputFormat = "html"
	OutputFormatMarkdown OutputFormat = "markdown"
)

type Config struct {
	OutputFormat OutputFormat
	ToPages      bool
}

type EPUBParser struct {
	outputFormat OutputFormat
	toPages      bool
}

type epubSection struct {
	sourcePath string
	bodyHTML   string
}

type epubContainer struct {
	Rootfiles []epubRootfile `xml:"rootfiles>rootfile"`
}

type epubRootfile struct {
	FullPath string `xml:"full-path,attr"`
}

type epubPackage struct {
	Manifest []epubManifestItem `xml:"manifest>item"`
	Spine    []epubSpineItemRef `xml:"spine>itemref"`
}

type epubManifestItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

type epubSpineItemRef struct {
	IDRef string `xml:"idref,attr"`
}

var _ einoparser.Parser = (*EPUBParser)(nil)

func NewEPUBParser(config *Config) *EPUBParser {
	parser := &EPUBParser{
		outputFormat: OutputFormatHTML,
		toPages:      false,
	}

	if config == nil {
		return parser
	}

	switch config.OutputFormat {
	case OutputFormatHTML, "":
		parser.outputFormat = OutputFormatHTML
	case OutputFormatMarkdown:
		parser.outputFormat = OutputFormatMarkdown
	default:
		parser.outputFormat = OutputFormatHTML
	}
	parser.toPages = config.ToPages

	return parser
}

func (p *EPUBParser) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("epub parser read all from reader failed: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("epub parser invalid zip archive: %w", err)
	}

	rootfilePath, err := resolveEPUBRootfilePath(zipReader)
	if err != nil {
		return nil, err
	}

	pkg, err := parseEPUBPackage(zipReader, rootfilePath)
	if err != nil {
		return nil, err
	}

	sections, err := readEPUBSections(ctx, zipReader, rootfilePath, pkg)
	if err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("epub parser found no readable html sections")
	}

	contentType := "text/html"
	commonOpts := einoparser.GetCommonOptions(nil, opts...)
	baseMeta := copyMeta(commonOpts.ExtraMeta)
	if commonOpts.URI != "" {
		baseMeta[einoparser.MetaKeySource] = commonOpts.URI
	}

	if p.outputFormat == OutputFormatMarkdown {
		contentType = "text/markdown"
	}

	if p.toPages {
		docs := make([]*schema.Document, 0, len(sections))
		for idx, section := range sections {
			content := buildEPUBHTMLSectionDocument(section, idx+1)
			if p.outputFormat == OutputFormatMarkdown {
				content, err = htmltomarkdown.ConvertString(content)
				if err != nil {
					return nil, fmt.Errorf("epub parser convert html to markdown failed at section %d: %w", idx+1, err)
				}
			}

			meta := copyMeta(baseMeta)
			meta["content_type"] = contentType
			meta["epub_section_index"] = idx + 1
			meta["epub_section_source"] = section.sourcePath

			docs = append(docs, &schema.Document{
				ID:       uuid.NewV4().String(),
				Content:  content,
				MetaData: meta,
			})
		}
		return docs, nil
	}

	content := buildEPUBHTMLDocument(sections)
	if p.outputFormat == OutputFormatMarkdown {
		content, err = htmltomarkdown.ConvertString(content)
		if err != nil {
			return nil, fmt.Errorf("epub parser convert html to markdown failed: %w", err)
		}
	}

	meta := copyMeta(baseMeta)
	meta["content_type"] = contentType

	return []*schema.Document{
		{
			ID:       uuid.NewV4().String(),
			Content:  content,
			MetaData: meta,
		},
	}, nil
}

func resolveEPUBRootfilePath(zipReader *zip.Reader) (string, error) {
	containerFile := findZipFile(zipReader, "META-INF/container.xml")
	if containerFile != nil {
		data, err := readZipFile(containerFile)
		if err == nil {
			var container epubContainer
			if err := xml.Unmarshal(data, &container); err == nil {
				for _, rootfile := range container.Rootfiles {
					candidate := path.Clean(strings.TrimSpace(rootfile.FullPath))
					if candidate != "" && candidate != "." {
						return candidate, nil
					}
				}
			}
		}
	}

	for _, f := range zipReader.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".opf") {
			return path.Clean(f.Name), nil
		}
	}

	return "", fmt.Errorf("epub parser root package file (.opf) not found")
}

func parseEPUBPackage(zipReader *zip.Reader, opfPath string) (*epubPackage, error) {
	opfFile := findZipFile(zipReader, opfPath)
	if opfFile == nil {
		return nil, fmt.Errorf("epub parser package file %q not found", opfPath)
	}

	data, err := readZipFile(opfFile)
	if err != nil {
		return nil, fmt.Errorf("epub parser read package file %q failed: %w", opfPath, err)
	}

	pkg := &epubPackage{}
	if err := xml.Unmarshal(data, pkg); err != nil {
		return nil, fmt.Errorf("epub parser decode package file %q failed: %w", opfPath, err)
	}

	return pkg, nil
}

func readEPUBSections(
	ctx context.Context,
	zipReader *zip.Reader,
	opfPath string,
	pkg *epubPackage,
) ([]epubSection, error) {
	manifestByID := make(map[string]epubManifestItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		manifestByID[item.ID] = item
	}

	baseDir := path.Dir(opfPath)
	sections := make([]epubSection, 0, len(pkg.Spine))
	appendSection := func(item epubManifestItem) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !isEPUBHTMLMediaType(item.MediaType) {
			return nil
		}

		href := strings.TrimSpace(strings.SplitN(item.Href, "#", 2)[0])
		if href == "" {
			return nil
		}

		targetPath := path.Clean(path.Join(baseDir, href))
		file := findZipFile(zipReader, targetPath)
		if file == nil {
			return nil
		}

		content, err := readZipFile(file)
		if err != nil {
			return fmt.Errorf("epub parser read chapter %q failed: %w", targetPath, err)
		}

		bodyHTML, err := extractHTMLBody(content)
		if err != nil {
			return fmt.Errorf("epub parser parse chapter %q failed: %w", targetPath, err)
		}
		if strings.TrimSpace(bodyHTML) == "" {
			return nil
		}

		sections = append(sections, epubSection{
			sourcePath: targetPath,
			bodyHTML:   bodyHTML,
		})
		return nil
	}

	for _, itemRef := range pkg.Spine {
		item, ok := manifestByID[itemRef.IDRef]
		if !ok {
			continue
		}
		if err := appendSection(item); err != nil {
			return nil, err
		}
	}

	if len(sections) > 0 {
		return sections, nil
	}

	for _, item := range pkg.Manifest {
		if err := appendSection(item); err != nil {
			return nil, err
		}
	}

	return sections, nil
}

func isEPUBHTMLMediaType(mediaType string) bool {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "application/xhtml+xml", "text/html":
		return true
	default:
		return false
	}
}

func findZipFile(zipReader *zip.Reader, targetPath string) *zip.File {
	cleanTarget := path.Clean(targetPath)
	for _, f := range zipReader.File {
		if path.Clean(f.Name) == cleanTarget {
			return f
		}
	}
	return nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

func extractHTMLBody(content []byte) (string, error) {
	root, err := xhtml.Parse(bytes.NewReader(content))
	if err != nil {
		return "", err
	}

	body := findHTMLNode(root, "body")
	if body == nil {
		var full bytes.Buffer
		if err := xhtml.Render(&full, root); err != nil {
			return "", err
		}
		return full.String(), nil
	}

	var bodyContent bytes.Buffer
	for node := body.FirstChild; node != nil; node = node.NextSibling {
		if err := xhtml.Render(&bodyContent, node); err != nil {
			return "", err
		}
	}

	return bodyContent.String(), nil
}

func findHTMLNode(node *xhtml.Node, targetTag string) *xhtml.Node {
	if node == nil {
		return nil
	}

	if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, targetTag) {
		return node
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findHTMLNode(child, targetTag); found != nil {
			return found
		}
	}

	return nil
}

func buildEPUBHTMLDocument(sections []epubSection) string {
	var builder strings.Builder
	builder.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"></head><body>")
	for idx, section := range sections {
		builder.WriteString("<section data-epub-index=\"")
		builder.WriteString(strconv.Itoa(idx + 1))
		builder.WriteString("\" data-epub-source=\"")
		builder.WriteString(stdhtml.EscapeString(section.sourcePath))
		builder.WriteString("\">")
		builder.WriteString(section.bodyHTML)
		builder.WriteString("</section>")
	}
	builder.WriteString("</body></html>")
	return builder.String()
}

func buildEPUBHTMLSectionDocument(section epubSection, sectionIndex int) string {
	var builder strings.Builder
	builder.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"></head><body>")
	builder.WriteString("<section data-epub-index=\"")
	builder.WriteString(strconv.Itoa(sectionIndex))
	builder.WriteString("\" data-epub-source=\"")
	builder.WriteString(stdhtml.EscapeString(section.sourcePath))
	builder.WriteString("\">")
	builder.WriteString(section.bodyHTML)
	builder.WriteString("</section>")
	builder.WriteString("</body></html>")
	return builder.String()
}

func copyMeta(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}

	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}
