package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// WriteXLSX writes a small, dependency-free XLSX workbook with one sheet.
// It is intentionally simple, but fully valid for Excel/WPS and avoids CSV encoding issues.
func WriteXLSX(w io.Writer, sheetName string, headers []string, rows [][]string) error {
	zw := zip.NewWriter(w)
	files := map[string]string{
		"[Content_Types].xml":        contentTypesXML,
		"_rels/.rels":                rootRelsXML,
		"xl/workbook.xml":            workbookXML(sheetName),
		"xl/_rels/workbook.xml.rels": workbookRelsXML,
		"xl/styles.xml":              stylesXML,
		"xl/worksheets/sheet1.xml":   worksheetXML(headers, rows),
		"docProps/core.xml":          coreXML,
		"docProps/app.xml":           appXML,
	}
	for name, content := range files {
		if err := addXLSXFile(zw, name, content); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func addXLSXFile(zw *zip.Writer, name string, content string) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetModTime(time.Now())
	fw, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = fw.Write([]byte(content))
	return err
}

func worksheetXML(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		headers = []string{"暂无数据"}
	}
	if len(rows) == 0 {
		rows = [][]string{make([]string, len(headers))}
	}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = utf8.RuneCountInString(header) + 6
	}
	for _, row := range rows {
		for i := range headers {
			value := ""
			if i < len(row) {
				value = row[i]
			}
			if width := utf8.RuneCountInString(value) + 4; width > widths[i] {
				widths[i] = width
			}
		}
	}
	for i := range widths {
		if widths[i] < 12 {
			widths[i] = 12
		}
		if widths[i] > 42 {
			widths[i] = 42
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	b.WriteString(`<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
	b.WriteString(`<cols>`)
	for i, width := range widths {
		b.WriteString(fmt.Sprintf(`<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, width))
	}
	b.WriteString(`</cols><sheetData>`)
	b.WriteString(`<row r="1" ht="24" customHeight="1">`)
	for i, header := range headers {
		b.WriteString(cellXML(i, 1, header, 1))
	}
	b.WriteString(`</row>`)
	for r, row := range rows {
		rowNumber := r + 2
		b.WriteString(fmt.Sprintf(`<row r="%d" ht="22" customHeight="1">`, rowNumber))
		for c := range headers {
			value := ""
			if c < len(row) {
				value = row[c]
			}
			b.WriteString(cellXML(c, rowNumber, value, 2))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func cellXML(column int, row int, value string, style int) string {
	return fmt.Sprintf(`<c r="%s%d" t="inlineStr" s="%d"><is><t>%s</t></is></c>`, excelColumn(column), row, style, xmlEscape(value))
}

func excelColumn(index int) string {
	name := ""
	n := index + 1
	for n > 0 {
		mod := (n - 1) % 26
		name = string(rune('A'+mod)) + name
		n = (n - mod) / 26
	}
	return name
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func workbookXML(sheetName string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="` + xmlEscape(sheetName) + `" sheetId="1" r:id="rId1"/></sheets></workbook>`
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="2"><font><sz val="11"/><name val="Microsoft YaHei"/></font><font><b/><sz val="11"/><name val="Microsoft YaHei"/><color rgb="FFFFFFFF"/></font></fonts>
  <fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF2563EB"/><bgColor indexed="64"/></patternFill></fill></fills>
  <borders count="2"><border/><border><left style="thin"><color rgb="FFD9E2F3"/></left><right style="thin"><color rgb="FFD9E2F3"/></right><top style="thin"><color rgb="FFD9E2F3"/></top><bottom style="thin"><color rgb="FFD9E2F3"/></bottom></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFill="1" applyFont="1" applyBorder="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1"/></cellXfs>
</styleSheet>`

const coreXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:creator>SaaS Admin</dc:creator>
  <cp:lastModifiedBy>SaaS Admin</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">2026-05-09T00:00:00Z</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">2026-05-09T00:00:00Z</dcterms:modified>
</cp:coreProperties>`

const appXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>SaaS Admin</Application>
</Properties>`
