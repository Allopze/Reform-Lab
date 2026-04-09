package ocrutil

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const defaultLanguage = "eng"

type BoundingBox struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Word struct {
	Text       string      `json:"text"`
	Confidence int         `json:"confidence"`
	Box        BoundingBox `json:"bbox"`
}

type Line struct {
	LineNumber int         `json:"lineNumber"`
	Text       string      `json:"text"`
	Confidence int         `json:"confidence"`
	Box        BoundingBox `json:"bbox"`
	Words      []Word      `json:"words"`
}

type Block struct {
	BlockNumber int         `json:"blockNumber"`
	Text        string      `json:"text"`
	Confidence  int         `json:"confidence"`
	Box         BoundingBox `json:"bbox"`
	Lines       []Line      `json:"lines"`
}

type Page struct {
	PageNumber int     `json:"pageNumber"`
	Text       string  `json:"text"`
	Blocks     []Block `json:"blocks"`
}

type Document struct {
	Text  string `json:"text"`
	Pages []Page `json:"pages"`
}

func RunTesseract(ctx context.Context, inputPath, outputBase, outputKind string) (string, error) {
	args := []string{inputPath, outputBase, "-l", defaultLanguage}
	ext := ".txt"
	switch outputKind {
	case "txt":
		// Plain text is the default output mode.
	case "tsv":
		args = append(args, "tsv")
		ext = ".tsv"
	case "pdf":
		args = append(args, "pdf")
		ext = ".pdf"
	default:
		return "", fmt.Errorf("unsupported tesseract output kind: %s", outputKind)
	}

	cmd := exec.CommandContext(ctx, "tesseract", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("tesseract %s: %s: %w", outputKind, strings.TrimSpace(string(out)), err)
	}

	return outputBase + ext, nil
}

func MergePDFs(ctx context.Context, outputPath string, inputs []string) error {
	if len(inputs) == 0 {
		return fmt.Errorf("no PDFs to merge")
	}
	if len(inputs) == 1 {
		return copyFile(outputPath, inputs[0])
	}

	args := []string{
		"-dBATCH",
		"-dNOPAUSE",
		"-q",
		"-sDEVICE=pdfwrite",
		"-sOutputFile=" + outputPath,
	}
	args = append(args, inputs...)

	cmd := exec.CommandContext(ctx, "gs", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ghostscript merge: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func ParseTSVFile(path string, pageNumberOverride int) (Page, error) {
	file, err := os.Open(path)
	if err != nil {
		return Page{}, fmt.Errorf("open tsv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return Page{}, fmt.Errorf("read tsv: %w", err)
	}
	if len(records) == 0 {
		return Page{PageNumber: positiveOr(pageNumberOverride, 1)}, nil
	}

	index := map[string]int{}
	for i, name := range records[0] {
		index[name] = i
	}

	page := &pageAccumulator{
		pageNumber: positiveOr(pageNumberOverride, 1),
		blocks:     map[int]*blockAccumulator{},
	}

	for _, record := range records[1:] {
		if len(record) == 0 {
			continue
		}
		level := parseColumn(record, index, "level")
		if level == 0 {
			continue
		}
		pageNum := parseColumn(record, index, "page_num")
		if pageNumberOverride > 0 {
			pageNum = pageNumberOverride
		}
		if pageNum > 0 {
			page.pageNumber = pageNum
		}

		blockNum := positiveOr(parseColumn(record, index, "block_num"), 1)
		lineNum := positiveOr(parseColumn(record, index, "line_num"), 1)
		conf := parseColumn(record, index, "conf")
		text := strings.TrimSpace(columnValue(record, index, "text"))
		box := BoundingBox{
			Left:   parseColumn(record, index, "left"),
			Top:    parseColumn(record, index, "top"),
			Width:  parseColumn(record, index, "width"),
			Height: parseColumn(record, index, "height"),
		}

		block := page.ensureBlock(blockNum, box)
		if level == 2 {
			continue
		}

		line := block.ensureLine(lineNum, box)
		if level == 4 {
			continue
		}
		if level != 5 || text == "" {
			continue
		}

		line.words = append(line.words, Word{Text: text, Confidence: conf, Box: box})
		line.box = mergeBoxes(line.box, box)
		block.box = mergeBoxes(block.box, box)
	}

	return page.build(), nil
}

func WriteDocumentJSON(outputPath string, pages []Page) error {
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].PageNumber < pages[j].PageNumber
	})

	textParts := make([]string, 0, len(pages))
	for _, page := range pages {
		if strings.TrimSpace(page.Text) != "" {
			textParts = append(textParts, page.Text)
		}
	}

	payload, err := json.MarshalIndent(Document{
		Text:  strings.Join(textParts, "\n\n"),
		Pages: pages,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ocr json: %w", err)
	}

	return os.WriteFile(outputPath, append(payload, '\n'), 0o644)
}

type pageAccumulator struct {
	pageNumber int
	blocks     map[int]*blockAccumulator
}

type blockAccumulator struct {
	blockNumber int
	box         BoundingBox
	lines       map[int]*lineAccumulator
}

type lineAccumulator struct {
	lineNumber int
	box        BoundingBox
	words      []Word
}

func (p *pageAccumulator) ensureBlock(blockNumber int, box BoundingBox) *blockAccumulator {
	block, ok := p.blocks[blockNumber]
	if !ok {
		block = &blockAccumulator{blockNumber: blockNumber, box: box, lines: map[int]*lineAccumulator{}}
		p.blocks[blockNumber] = block
		return block
	}
	block.box = mergeBoxes(block.box, box)
	return block
}

func (b *blockAccumulator) ensureLine(lineNumber int, box BoundingBox) *lineAccumulator {
	line, ok := b.lines[lineNumber]
	if !ok {
		line = &lineAccumulator{lineNumber: lineNumber, box: box}
		b.lines[lineNumber] = line
		return line
	}
	line.box = mergeBoxes(line.box, box)
	return line
}

func (p *pageAccumulator) build() Page {
	blockNumbers := sortedKeys(p.blocks)
	blocks := make([]Block, 0, len(blockNumbers))
	pageText := make([]string, 0, len(blockNumbers))

	for _, blockNumber := range blockNumbers {
		block := p.blocks[blockNumber]
		lineNumbers := sortedKeys(block.lines)
		lines := make([]Line, 0, len(lineNumbers))
		blockText := make([]string, 0, len(lineNumbers))
		blockConfidenceSum := 0
		blockConfidenceCount := 0

		for _, lineNumber := range lineNumbers {
			lineAcc := block.lines[lineNumber]
			words := append([]Word(nil), lineAcc.words...)
			lineWords := make([]string, 0, len(words))
			lineConfidenceSum := 0
			lineConfidenceCount := 0
			for _, word := range words {
				lineWords = append(lineWords, word.Text)
				if word.Confidence >= 0 {
					lineConfidenceSum += word.Confidence
					lineConfidenceCount++
				}
			}

			text := strings.Join(lineWords, " ")
			confidence := averageConfidence(lineConfidenceSum, lineConfidenceCount)
			if confidence >= 0 {
				blockConfidenceSum += confidence
				blockConfidenceCount++
			}
			if text != "" {
				blockText = append(blockText, text)
			}

			lines = append(lines, Line{
				LineNumber: lineNumber,
				Text:       text,
				Confidence: confidence,
				Box:        lineAcc.box,
				Words:      words,
			})
		}

		text := strings.Join(blockText, "\n")
		if text != "" {
			pageText = append(pageText, text)
		}

		blocks = append(blocks, Block{
			BlockNumber: blockNumber,
			Text:        text,
			Confidence:  averageConfidence(blockConfidenceSum, blockConfidenceCount),
			Box:         block.box,
			Lines:       lines,
		})
	}

	return Page{
		PageNumber: positiveOr(p.pageNumber, 1),
		Text:       strings.Join(pageText, "\n\n"),
		Blocks:     blocks,
	}
}

func sortedKeys[T any](items map[int]T) []int {
	keys := make([]int, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func mergeBoxes(current, next BoundingBox) BoundingBox {
	if isZeroBox(current) {
		return next
	}
	if isZeroBox(next) {
		return current
	}
	left := min(current.Left, next.Left)
	top := min(current.Top, next.Top)
	right := max(current.Left+current.Width, next.Left+next.Width)
	bottom := max(current.Top+current.Height, next.Top+next.Height)
	return BoundingBox{
		Left:   left,
		Top:    top,
		Width:  right - left,
		Height: bottom - top,
	}
}

func isZeroBox(box BoundingBox) bool {
	return box.Width == 0 && box.Height == 0
}

func averageConfidence(sum, count int) int {
	if count == 0 {
		return -1
	}
	return sum / count
}

func parseColumn(record []string, index map[string]int, key string) int {
	value := columnValue(record, index, key)
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func columnValue(record []string, index map[string]int, key string) string {
	position, ok := index[key]
	if !ok || position >= len(record) {
		return ""
	}
	return record[position]
}

func positiveOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func OutputBase(outputDir, name string) string {
	return filepath.Join(outputDir, name)
}
