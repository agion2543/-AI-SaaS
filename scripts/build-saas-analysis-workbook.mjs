import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const outputDir = "C:/Users/Administrator/Documents/Codex/2026-04-27/go-web-gin-health/outputs";
const outputPath = path.join(outputDir, "本地生活商家AI运营SaaS-方案梳理与优化路线图.xlsx");

const wb = Workbook.create();

const palette = {
  navy: "#0F172A",
  blue: "#2563EB",
  sky: "#EAF4FF",
  green: "#16A34A",
  amber: "#F59E0B",
  red: "#DC2626",
  gray: "#64748B",
  border: "#D9E2F3",
  white: "#FFFFFF",
  soft: "#F8FBFF"
};

const widths = {
  A: 18,
  B: 26,
  C: 36,
  D: 34,
  E: 34,
  F: 28,
  G: 26,
  H: 26
};

const moneyFmt = "¥#,##0";
const pctFmt = "0%";

function setWidths(sheet, maxCol = "H") {
  const letters = Object.keys(widths).filter((letter) => letter <= maxCol);
  for (const letter of letters) sheet.getRange(`${letter}:${letter}`).format.columnWidth = widths[letter];
}

function styleSheet(sheet) {
  sheet.showGridLines = false;
  setWidths(sheet);
  sheet.getRange("A1:H1").format = {
    fill: palette.navy,
    font: { color: palette.white, bold: true, size: 18 },
    wrapText: true
  };
  sheet.getRange("A2:H2").format = {
    fill: palette.sky,
    font: { color: palette.navy, bold: true },
    wrapText: true
  };
  sheet.freezePanes.freezeRows(2);
}

function writeTitle(sheet, title, subtitle) {
  sheet.getRange("A1:H1").merge();
  sheet.getRange("A1").values = [[title]];
  sheet.getRange("A2:H2").merge();
  sheet.getRange("A2").values = [[subtitle]];
}

function writeTable(sheet, startCell, headers, rows) {
  const start = sheet.getRange(startCell);
  const rowCount = rows.length + 1;
  const colCount = headers.length;
  const range = start.resize(rowCount, colCount);
  range.values = [headers, ...rows];
  range.format.wrapText = true;
  range.format.borders = {
    insideHorizontal: { style: "continuous", color: palette.border },
    insideVertical: { style: "continuous", color: palette.border },
    edgeBottom: { style: "continuous", color: palette.border },
    edgeTop: { style: "continuous", color: palette.border },
    edgeLeft: { style: "continuous", color: palette.border },
    edgeRight: { style: "continuous", color: palette.border }
  };
  start.resize(1, colCount).format = {
    fill: palette.blue,
    font: { color: palette.white, bold: true },
    wrapText: true
  };
  return range;
}

function kpi(sheet, cell, label, value, note, color = palette.blue) {
  const r = sheet.getRange(cell).resize(4, 2);
  r.merge();
  r.format = {
    fill: palette.white,
    font: { color: palette.navy, bold: true },
    borders: {
      edgeBottom: { style: "continuous", color: palette.border },
      edgeTop: { style: "continuous", color: palette.border },
      edgeLeft: { style: "continuous", color: palette.border },
      edgeRight: { style: "continuous", color: palette.border }
    },
    wrapText: true
  };
  const top = sheet.getRange(cell);
  top.values = [[`${label}\n${value}\n${note}`]];
  top.format.font = { color, bold: true, size: 14 };
}

const dashboard = wb.worksheets.add("总览");
styleSheet(dashboard);
writeTitle(dashboard, "本地生活商家 AI 运营 SaaS：方案梳理与优化路线图", "定位：面向本地商家提供门店、商品、扫码点单、订单财务、优惠活动与 AI 经营分析的一体化 SaaS。");
kpi(dashboard, "A4", "已覆盖端口", "平台端 / 商家端 / 顾客端", "三端闭环已具备基础雏形", palette.blue);
kpi(dashboard, "C4", "交易闭环", "顾客点单 → 支付 → 商家接单", "已具备订单与财务留痕", palette.green);
kpi(dashboard, "E4", "商业化方向", "订阅 SaaS + 人工结算", "短期更稳，避开自动分账风险", palette.amber);
kpi(dashboard, "G4", "下一阶段", "财务/订单/顾客体验补强", "先补基础闭环，再做部署", palette.red);
writeTable(dashboard, "A10", ["优先级", "模块", "当前判断", "下一步动作", "验收标准"], [
  ["P0", "财务与结算", "已有商家财务、平台财务、退款留痕、人工结算基础，但还需要更完整的对账视图。", "继续完善 XLSX 导出、结算记录、退款记录、订单来源区分。", "导出可被 WPS/Excel 正常打开，金额口径不混入商家订阅订单。"],
  ["P0", "订单闭环", "顾客点单、购物车、创建订单、商家接单已成型。", "强化顾客订单状态页、支付失败重试、商家订单详情和售后记录。", "顾客能看到订单状态，商家能处理接单/完成/退款。"],
  ["P1", "经营看板", "已有基础经营看板和 AI 分析雏形。", "接入真实订单、热销/低动销商品、客单价、退款率、取消率。", "商家登录后能看懂今日经营结果和下一步建议。"],
  ["P1", "商品与活动", "已有商品管理、优惠活动和活动折扣基础。", "补分类、图片上传、上下架排序、优惠券真正抵扣订单金额。", "顾客端按分类点单，活动能影响实付金额。"],
  ["P2", "上线部署", "已有安全检查和修复向导，但功能仍在优化中。", "等核心业务稳定后整理部署包、Nginx、HTTPS、备份、生产 .env。", "能在服务器按文档部署并通过安全检查。"]
]);
dashboard.getRange("G16:H22").values = [
  ["优先级分布", "数量"],
  ["P0", 2],
  ["P1", 2],
  ["P2", 1],
  ["已完成基础", 8],
  ["半完成", 7],
  ["待新增", 9]
];
dashboard.getRange("H17:H23").format.numberFormat = "0";
const chart = dashboard.charts.add("bar", dashboard.getRange("G16:H23"));
chart.title = "当前优化优先级分布";
chart.hasLegend = false;
chart.xAxis = { axisType: "textAxis" };
chart.setPosition("F10", "H24");

const featureMap = wb.worksheets.add("三端功能地图");
styleSheet(featureMap);
writeTitle(featureMap, "三端功能地图", "把平台方、商家、顾客三个角色拆清楚，避免把商家用户和顾客用户混淆。");
writeTable(featureMap, "A4", ["端口", "角色", "已完成/已有雏形", "半完成", "需要新增/优化", "业务价值"], [
  ["平台端", "平台运营方", "登录、商家管理、商家详情、订阅开通、订单财务、退款入口、人工结算、安全检查、审计日志。", "运营总览、商家风控、财务统计已有基础但仍需细化。", "结算审核流、异常商家预警、配置修复向导继续增强、部署清单。", "平台能管理商家、订阅、交易流水、风险和上线安全。"],
  ["商家端", "门店/企业客户", "注册登录、订阅、门店、商品、订单、财务、收款设置、优惠活动、AI 经营分析。", "经营看板、优惠活动、财务对账和 AI 建议已成型但数据深度不足。", "商品分类、图片上传、订单打印、小票、真实退款对接、活动自动抵扣。", "商家能日常经营、看数据、做活动、处理订单售后。"],
  ["顾客端", "扫码点单消费者", "门店页、商品展示、购物车、下单支付、订单成功页。", "移动端体验已优化过，但订单状态页和支付失败重试仍需加强。", "订单状态页、支付失败重试、评价、会员/优惠券领取、顾客画像沉淀。", "顾客扫码即可完成下单，最终形成经营数据来源。"]
]);

const roadmap = wb.worksheets.add("优化路线图");
styleSheet(roadmap);
writeTitle(roadmap, "后续优化与新增功能路线图", "优先顺序：先把真实业务闭环做稳，再增加 AI 与部署能力。");
writeTable(roadmap, "A4", ["阶段", "优先级", "任务", "主要内容", "依赖", "验收方式"], [
  ["第 1 阶段", "P0", "财务导出与对账稳定", "所有 CSV 改 XLSX；平台/商家财务口径统一；订单金额、退款、净收入分开展示。", "订单、退款、结算数据", "WPS/Excel 正常打开，金额与页面一致。"],
  ["第 1 阶段", "P0", "顾客订单状态页", "顾客支付后查看待接单、已接单、已完成、已退款等状态；支付失败可重试。", "订单状态机", "顾客端可完整追踪订单。"],
  ["第 2 阶段", "P1", "商家经营看板增强", "今日订单、实收、退款、客单价、热销商品、低动销商品、门店提醒。", "真实订单数据", "商家看板能指导经营动作。"],
  ["第 2 阶段", "P1", "商品与活动深化", "商品分类、图片上传、上下架排序；优惠券/满减接入下单金额。", "商品表、活动表、订单计算", "顾客下单金额能自动抵扣。"],
  ["第 3 阶段", "P1", "AI 经营分析可操作化", "一键转优惠活动、新客复购券、沉睡召回券、高价值专属券。", "顾客画像、订单历史、活动模块", "AI 建议能生成可发布活动草稿。"],
  ["第 3 阶段", "P2", "平台风控与审计", "异常退款率、取消率、低接单率商家预警；关键后台操作留痕。", "审计日志、订单统计", "平台能追踪谁做了什么操作。"],
  ["第 4 阶段", "P2", "生产部署包", ".env.production、Nginx、HTTPS、备份、迁移顺序、启动脚本、上线检查。", "核心功能稳定", "可按文档部署到服务器。"]
]);

const differentiation = wb.worksheets.add("差异化分析");
styleSheet(differentiation);
writeTitle(differentiation, "产品差异化分析", "核心差异：不是单纯收银或会员系统，而是以本地商家的经营增长为目标，把交易数据沉淀为 AI 运营建议。");
writeTable(differentiation, "A4", ["维度", "传统收银/点单系统", "普通 SaaS 管理后台", "本项目差异化", "可形成的壁垒"], [
  ["目标客户", "主要服务单店收银或点单。", "偏通用管理流程。", "面向餐饮、服务、零售等本地生活商家，围绕经营增长设计。", "场景理解和行业模板沉淀。"],
  ["数据价值", "记录订单和流水。", "展示报表。", "订单、商品、活动、顾客画像统一沉淀，反哺 AI 建议。", "越用数据越准，运营建议越贴合。"],
  ["AI 能力", "通常没有或只是客服。", "多停留在文本生成。", "AI 建议直接转成优惠活动、召回券、满减策略。", "从“建议”到“动作”的闭环。"],
  ["商业模式", "硬件/系统一次性或低月费。", "订阅为主。", "商家订阅 + 人工结算 + 后续服务商/分账扩展。", "先轻量上线，后续可扩展金融与营销服务。"],
  ["平台视角", "商家自己用。", "管理员看基础列表。", "平台能看商家经营、订阅、风险、顾客流转和结算状态。", "适合做区域运营平台或行业服务商。"]
]);

const achievements = wb.worksheets.add("实践成果");
styleSheet(achievements);
writeTitle(achievements, "当前实践成绩与可展示成果", "这些是目前已经能在项目里看到或测试的成果，也是后续对外演示的基础。");
writeTable(achievements, "A4", ["类别", "已完成成果", "可展示页面/模块", "实际应用意义", "后续增强"], [
  ["多角色体系", "平台端、商家端、顾客端已拆分，商家订阅权限已接入。", "/admin、/merchant、/customer/store/:id", "具备 SaaS 平台基础架构。", "进一步补员工角色和多门店总管理。"],
  ["交易闭环", "顾客可扫码进入门店，查看商品、加入购物车、创建订单，商家端可接单。", "顾客门店页、商家订单管理", "能模拟真实门店下单流程。", "补支付失败重试、订单状态页和评价。"],
  ["财务闭环", "商家财务、平台订单财务、退款留痕、人工结算基础已建。", "商家财务、平台订单财务、商家详情结算记录", "方便对账和人工打款。", "继续完善 XLSX 导出、结算单明细和真实退款。"],
  ["经营工具", "商品管理、优惠活动、AI 经营分析已有雏形。", "商品管理、优惠活动、AI 经营分析", "能体现“运营赋能”而不只是点单。", "AI 一键生成活动并接入真实抵扣。"],
  ["上线准备", "安全检查面板、修复向导、备份脚本、生产配置模板已有。", "系统配置/安全检查", "开始具备部署意识。", "等功能稳定后整理完整部署包。"]
]);

const vision = wb.worksheets.add("方案愿景");
styleSheet(vision);
writeTitle(vision, "实践愿景与实际应用路径", "愿景不是先做大而全，而是从单店可用的经营工具出发，逐步升级为本地生活商家 AI 运营平台。");
writeTable(vision, "A4", ["阶段", "产品形态", "目标用户", "关键能力", "收入方式", "阶段成果"], [
  ["MVP", "单商家/单门店 SaaS", "餐饮、烧烤、服务类小商家", "扫码点单、商品、订单、财务、订阅。", "月付/年付订阅，平台人工结算。", "可演示完整交易闭环。"],
  ["增长版", "商家经营工作台", "有复购和营销需求的商家", "经营看板、优惠活动、顾客标签、AI 建议。", "订阅 + 增值 AI 模块。", "商家每天能看数据并执行活动。"],
  ["区域版", "平台运营后台", "区域运营商、本地服务商", "商家排行、风控、结算、巡检、审计。", "服务费、代运营、行业模板。", "可管理多个商家并形成运营数据。"],
  ["成熟版", "合规支付/分账平台", "具备资质的平台方和商家", "服务商/分账、真实退款、商户入驻审核。", "SaaS + 交易服务 + 增值营销。", "自动化资金流与合规经营。"]
]);

const business = wb.worksheets.add("商业与合规");
styleSheet(business);
writeTitle(business, "商业化落地与支付合规建议", "现阶段重点是避开自动分账和二清风险，用人工结算与清晰财务报表支撑早期验证。");
writeTable(business, "A4", ["主题", "当前方案", "原因", "风险", "建议路径"], [
  ["真实收款", "短期不建议对真实用户开放未资质化在线收款。", "支付宝/微信正式商家收款通常需要经营主体资质。", "无资质收款可能涉及账户、税务、投诉和合规风险。", "先使用沙箱/模拟支付；真实业务前办理个体工商户或公司资质。"],
  ["平台统一收款", "技术上可做，但真实上线前需平台有合法主体。", "平台统一收款再给商家结算，需要清晰合同和账务。", "大规模自动分账可能触及二清风险。", "早期采用人工结算，后续走官方服务商/分账产品。"],
  ["商家直收", "长期更理想，但接入复杂。", "资金直接进入商家账户，平台只做 SaaS 和数据。", "需要商家提交资料、开通子商户或分账能力。", "成熟后申请服务商或官方分账接口。"],
  ["退款", "当前先做退款记录留痕。", "便于财务核对和售后流程验证。", "若真实支付未同步退款，会形成账实不一致。", "接支付宝沙箱退款，再接正式退款接口。"],
  ["对账", "导出 XLSX，平台和商家分别核对。", "早期人工结算最需要清晰报表。", "CSV 容易乱码且不适合财务人员使用。", "继续完善结算单、订单明细、退款明细和导出模板。"]
]);

const risks = wb.worksheets.add("风险与检查");
styleSheet(risks);
writeTitle(risks, "关键风险与上线前检查", "这张表用于每天开发后复盘：哪些风险会阻碍真实上线，哪些只是体验优化。");
writeTable(risks, "A4", ["风险", "当前状态", "影响", "优先级", "处理建议"], [
  ["支付资质", "未具备正式商家支付资质。", "无法安全进行真实线上收款。", "P0", "继续沙箱/模拟测试；真实上线前办理合法主体并开通支付产品。"],
  ["订单状态页", "顾客端订单后续追踪仍需增强。", "顾客支付后体验不完整。", "P0", "补订单状态页、失败重试、状态刷新。"],
  ["财务导出", "本轮已改为 XLSX。", "降低 WPS 乱码和财务核对障碍。", "P0", "继续验证各页面导出内容是否完整。"],
  ["优惠抵扣", "活动展示已有，真实抵扣仍需深化。", "营销功能不够闭环。", "P1", "把优惠券/满减接入订单计算。"],
  ["AI 可执行性", "已有分析和建议，但自动转活动需继续打通。", "AI 价值不够显性。", "P1", "根据顾客标签生成活动草稿并一键发布。"],
  ["部署安全", "安全检查面板已有，生产部署包未最终整理。", "上线配置容易漏项。", "P2", "核心功能稳定后整理部署包。"]
]);

for (const sheet of wb.worksheets.items) {
  sheet.getUsedRange()?.format.autofitRows();
}

const inspection = await wb.inspect({
  kind: "table",
  range: "总览!A1:H18",
  include: "values",
  tableMaxRows: 18,
  tableMaxCols: 8
});
console.log(inspection.ndjson.slice(0, 1200));

const errors = await wb.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 100 },
  summary: "final formula error scan"
});
console.log(errors.ndjson);

const previewDir = path.join(outputDir, "preview");
await fs.mkdir(previewDir, { recursive: true });
for (const sheet of wb.worksheets.items) {
  const preview = await wb.render({ sheetName: sheet.name, autoCrop: "all", scale: 1, format: "png" });
  await fs.writeFile(path.join(previewDir, `${sheet.name}.png`), new Uint8Array(await preview.arrayBuffer()));
}

await fs.mkdir(outputDir, { recursive: true });
const xlsx = await SpreadsheetFile.exportXlsx(wb);
await xlsx.save(outputPath);
console.log(outputPath);
