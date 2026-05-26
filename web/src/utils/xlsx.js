const encoder = new TextEncoder()

const crcTable = (() => {
  const table = new Uint32Array(256)
  for (let i = 0; i < 256; i += 1) {
    let c = i
    for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    table[i] = c >>> 0
  }
  return table
})()

const crc32 = (bytes) => {
  let crc = 0xffffffff
  for (const byte of bytes) crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8)
  return (crc ^ 0xffffffff) >>> 0
}

const writeU16 = (arr, value) => arr.push(value & 0xff, (value >>> 8) & 0xff)
const writeU32 = (arr, value) => arr.push(value & 0xff, (value >>> 8) & 0xff, (value >>> 16) & 0xff, (value >>> 24) & 0xff)

const dosTime = () => {
  const d = new Date()
  return {
    time: (d.getHours() << 11) | (d.getMinutes() << 5) | Math.floor(d.getSeconds() / 2),
    date: ((d.getFullYear() - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate()
  }
}

const zipStore = (files) => {
  const bytes = []
  const central = []
  const stamp = dosTime()
  let offset = 0

  files.forEach((file) => {
    const name = encoder.encode(file.name)
    const data = encoder.encode(file.content)
    const crc = crc32(data)

    writeU32(bytes, 0x04034b50)
    writeU16(bytes, 20)
    writeU16(bytes, 0)
    writeU16(bytes, 0)
    writeU16(bytes, stamp.time)
    writeU16(bytes, stamp.date)
    writeU32(bytes, crc)
    writeU32(bytes, data.length)
    writeU32(bytes, data.length)
    writeU16(bytes, name.length)
    writeU16(bytes, 0)
    bytes.push(...name, ...data)

    writeU32(central, 0x02014b50)
    writeU16(central, 20)
    writeU16(central, 20)
    writeU16(central, 0)
    writeU16(central, 0)
    writeU16(central, stamp.time)
    writeU16(central, stamp.date)
    writeU32(central, crc)
    writeU32(central, data.length)
    writeU32(central, data.length)
    writeU16(central, name.length)
    writeU16(central, 0)
    writeU16(central, 0)
    writeU16(central, 0)
    writeU16(central, 0)
    writeU32(central, 0)
    writeU32(central, offset)
    central.push(...name)

    offset = bytes.length
  })

  const centralOffset = bytes.length
  bytes.push(...central)
  writeU32(bytes, 0x06054b50)
  writeU16(bytes, 0)
  writeU16(bytes, 0)
  writeU16(bytes, files.length)
  writeU16(bytes, files.length)
  writeU32(bytes, central.length)
  writeU32(bytes, centralOffset)
  writeU16(bytes, 0)
  return new Uint8Array(bytes)
}

const xml = (value) => String(value ?? '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&apos;')

const columnName = (index) => {
  let name = ''
  let n = index + 1
  while (n > 0) {
    const mod = (n - 1) % 26
    name = String.fromCharCode(65 + mod) + name
    n = Math.floor((n - mod) / 26)
  }
  return name
}

const normalizeRows = (rows) => {
  const body = rows.length ? rows : [{ 暂无数据: '' }]
  const headers = Object.keys(body[0])
  return { headers, body }
}

const buildSheet = (rows) => {
  const { headers, body } = normalizeRows(rows)
  const widths = headers.map((header) => {
    const max = Math.max(String(header).length, ...body.map((row) => String(row[header] ?? '').length))
    return Math.min(Math.max(max + 4, 12), 42)
  })
  const headerRow = `<row r="1" ht="24" customHeight="1">${headers.map((header, index) => `<c r="${columnName(index)}1" t="inlineStr" s="1"><is><t>${xml(header)}</t></is></c>`).join('')}</row>`
  const dataRows = body.map((row, rowIndex) => {
    const r = rowIndex + 2
    return `<row r="${r}" ht="22" customHeight="1">${headers.map((header, index) => `<c r="${columnName(index)}${r}" t="inlineStr" s="2"><is><t>${xml(row[header])}</t></is></c>`).join('')}</row>`
  }).join('')
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>
  <cols>${widths.map((width, index) => `<col min="${index + 1}" max="${index + 1}" width="${width}" customWidth="1"/>`).join('')}</cols>
  <sheetData>${headerRow}${dataRows}</sheetData>
</worksheet>`
}

const workbookXml = (sheetName) => `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="${xml(sheetName)}" sheetId="1" r:id="rId1"/></sheets>
</workbook>`

const stylesXml = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="2"><font><sz val="11"/><name val="Microsoft YaHei"/></font><font><b/><sz val="11"/><name val="Microsoft YaHei"/><color rgb="FFFFFFFF"/></font></fonts>
  <fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF2563EB"/><bgColor indexed="64"/></patternFill></fill></fills>
  <borders count="2"><border/><border><left style="thin"><color rgb="FFD9E2F3"/></left><right style="thin"><color rgb="FFD9E2F3"/></right><top style="thin"><color rgb="FFD9E2F3"/></top><bottom style="thin"><color rgb="FFD9E2F3"/></bottom></border></borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFill="1" applyFont="1" applyBorder="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1"/></cellXfs>
</styleSheet>`

export const exportRowsToXlsx = (filename, sheetName, rows) => {
  const files = [
    { name: '[Content_Types].xml', content: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>` },
    { name: '_rels/.rels', content: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>` },
    { name: 'xl/workbook.xml', content: workbookXml(sheetName) },
    { name: 'xl/_rels/workbook.xml.rels', content: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>` },
    { name: 'xl/styles.xml', content: stylesXml },
    { name: 'xl/worksheets/sheet1.xml', content: buildSheet(rows) }
  ]
  const blob = new Blob([zipStore(files)], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename.endsWith('.xlsx') ? filename : `${filename}.xlsx`
  link.click()
  URL.revokeObjectURL(link.href)
}
