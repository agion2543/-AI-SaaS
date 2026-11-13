<template>
  <div class="ai-page">
    <section class="hero">
      <div class="hero-copy">
        <div class="eyebrow">MERCHANT AI WORKSPACE · SCAFFOLD</div>
        <h1>AI 智能经营模块</h1>
        <p>先保留模块边界和后续接入位置。当前没有连接真实 AI provider/API，不读取门店经营数据、不生成经营结论，也不会创建或发布营销活动。</p>
      </div>
      <div class="status-card">
        <span class="status-dot"></span>
        <div>
          <strong>当前状态：未接入</strong>
          <small>默认禁用 · 无外部模型调用</small>
        </div>
      </div>
    </section>

    <el-alert
      class="boundary-alert"
      type="warning"
      show-icon
      :closable="false"
      title="此页仅展示后续产品框架，不是已上线的 AI 能力"
      description="这里没有真实商家样本或线上验证结果；页面不展示模板生成的假建议。接入真实数据和模型前，所有分析、问答及活动建议均保持不可用。"
    />

    <section class="module-grid" aria-label="AI 经营模块预留能力">
      <button
        v-for="item in modules"
        :key="item.key"
        type="button"
        :class="['module-card', { active: activeModule === item.key }]"
        @click="activeModule = item.key"
      >
        <div class="module-topline">
          <span class="module-index">{{ item.index }}</span>
          <el-tag type="info" effect="plain">待接入</el-tag>
        </div>
        <h2>{{ item.title }}</h2>
        <p>{{ item.description }}</p>
        <small>{{ item.note }}</small>
      </button>
    </section>

    <section class="workspace-card">
      <div class="workspace-heading">
        <div>
          <span class="eyebrow dark">MODULE BLUEPRINT</span>
          <h2>{{ currentModule.title }}</h2>
          <p>{{ currentModule.intro }}</p>
        </div>
        <el-tag type="warning" effect="light">未开放</el-tag>
      </div>

      <div v-if="activeModule === 'analysis'" class="blueprint-grid">
        <article class="blueprint-block">
          <span>计划数据范围</span>
          <strong>订单 · 退款 · 商品 · 券核销</strong>
          <p>先确认字段含义、统计口径与商家授权范围；每条结论需要能追溯到来源和时间范围。</p>
        </article>
        <article class="blueprint-block">
          <span>当前数据状态</span>
          <strong>未连接业务数据源</strong>
          <p>本页不读取门店记录，因此不会用空值或演示值伪装成经营分析。</p>
        </article>
      </div>

      <div v-else-if="activeModule === 'qa'" class="qa-placeholder">
        <label for="merchant-ai-question">经营问答输入框（预留）</label>
        <el-input id="merchant-ai-question" disabled placeholder="provider 接入并完成数据权限校验后开放" />
        <p>未来回答只使用当前商家已授权的数据上下文，并显示数据来源、统计区间和不确定项。</p>
      </div>

      <div v-else class="activity-flow">
        <div v-for="step in activitySteps" :key="step.title" class="flow-step">
          <span>{{ step.index }}</span>
          <div>
            <strong>{{ step.title }}</strong>
            <p>{{ step.description }}</p>
          </div>
        </div>
        <el-alert
          type="info"
          show-icon
          :closable="false"
          title="当前不会生成活动草稿、修改优惠或对外发布"
        />
      </div>
    </section>

    <section class="integration-card">
      <div class="integration-heading">
        <span class="eyebrow dark">FUTURE INTEGRATION CHECKPOINTS</span>
        <h2>后续按阶段接入</h2>
        <p>等主系统和真实样本准备好后逐项验证，不因界面已预留就视为功能已验证。</p>
      </div>
      <div class="checkpoint-grid">
        <article v-for="item in checkpoints" :key="item.index">
          <span>{{ item.index }}</span>
          <strong>{{ item.title }}</strong>
          <small>{{ item.description }}</small>
        </article>
      </div>
      <div class="approval-note">
        <strong>人工确认边界</strong>
        <p>未来 AI 输出先作为可追溯建议或草稿；任何改价、发券、创建/发布活动等有经营影响的动作，都必须由商家确认后才能继续。</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const activeModule = ref('analysis')

const modules = [
  {
    key: 'analysis',
    index: '01',
    title: '数据驱动经营分析',
    description: '预留日报、退款与商品表现的分析入口。',
    note: '前置条件：授权数据、字段口径、可追溯来源',
    intro: '先建立可信的数据上下文，再考虑生成分析；没有真实样本时不显示结论。'
  },
  {
    key: 'qa',
    index: '02',
    title: '商家经营问答',
    description: '预留围绕门店业务数据的问答入口。',
    note: '前置条件：商家隔离、访问控制、隐私评估',
    intro: '问答需要约束到当前商家获准访问的数据，并能说明回答依据。'
  },
  {
    key: 'activity',
    index: '03',
    title: '活动建议与草稿',
    description: '预留从目标、成本到人工审核的建议流程。',
    note: '前置条件：成本边界、审核确认、发布权限',
    intro: '活动建议与真实创建/发布分开，AI 不能绕过商家确认直接执行。'
  }
]

const currentModule = computed(() => modules.find((item) => item.key === activeModule.value) || modules[0])

const activitySteps = [
  { index: '1', title: '读取已授权经营数据', description: '明确门店、时间范围、字段来源和统计口径。' },
  { index: '2', title: '生成可解释的建议草稿', description: '标注依据、模型版本、生成时间和不确定性。' },
  { index: '3', title: '商家检查并确认', description: '确认活动对象、优惠力度、预算及履约能力。' },
  { index: '4', title: '由商家手动执行', description: '发布或变更仍需商家操作，后台保留审计记录。' }
]

const checkpoints = [
  { index: 'A', title: '数据与授权', description: '核对订单、退款、核销和商品字段映射及来源。' },
  { index: 'B', title: 'Provider Adapter', description: '隔离供应商协议；凭据放密钥管理，不由页面收集。' },
  { index: 'C', title: '审计与成本', description: '记录上下文来源、模型/版本、结果、失败、耗时与成本。' },
  { index: 'D', title: '合成到真实验证', description: '先合成样本测试，再经商家授权做小范围验证。' }
]
</script>

<style scoped>
.ai-page {
  width: min(100%, 1440px);
  margin: 0 auto;
  display: grid;
  gap: 18px;
}
.hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 28px;
  padding: 30px;
  border: 1px solid rgba(147, 197, 253, .34);
  border-radius: 24px;
  background: radial-gradient(circle at 92% 4%, rgba(6, 182, 212, .32), transparent 28%), linear-gradient(125deg, #071a33, #124e87 68%, #0f766e);
  color: #fff;
}
.hero-copy { max-width: 850px; }
.eyebrow { color: #a7f3d0; font-size: 11px; font-weight: 900; letter-spacing: .15em; }
.eyebrow.dark { color: #2563eb; }
h1 { margin: 12px 0; font-size: clamp(30px, 4vw, 44px); letter-spacing: -.04em; }
.hero p { max-width: 850px; margin: 0; color: #dbeafe; font-size: 15px; line-height: 1.8; }
.status-card { min-width: 230px; display: flex; align-items: center; gap: 12px; padding: 16px; border: 1px solid rgba(255,255,255,.2); border-radius: 16px; background: rgba(255,255,255,.1); }
.status-card strong, .status-card small { display: block; }
.status-card strong { font-size: 15px; }
.status-card small { margin-top: 5px; color: #cbd5e1; }
.status-dot { width: 10px; height: 10px; flex: 0 0 auto; border-radius: 50%; background: #fbbf24; box-shadow: 0 0 0 5px rgba(251,191,36,.14); }
.boundary-alert { border-radius: 14px; }
.module-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.module-card { min-height: 182px; padding: 20px; border: 1px solid #dbe7f3; border-radius: 18px; background: #fff; text-align: left; cursor: pointer; transition: border-color .18s, transform .18s, box-shadow .18s; }
.module-card:hover, .module-card.active { border-color: #60a5fa; box-shadow: 0 14px 34px rgba(37,99,235,.1); transform: translateY(-1px); }
.module-topline { display: flex; justify-content: space-between; align-items: center; }
.module-index { color: #2563eb; font-size: 12px; font-weight: 900; letter-spacing: .12em; }
.module-card h2 { margin: 18px 0 8px; color: #102a43; font-size: 19px; }
.module-card p, .module-card small { display: block; margin: 0; color: #64748b; line-height: 1.6; }
.module-card small { margin-top: 10px; color: #475569; font-size: 12px; }
.workspace-card, .integration-card { padding: 22px; border: 1px solid #dbe7f3; border-radius: 18px; background: #fff; box-shadow: 0 12px 30px rgba(15,39,71,.04); }
.workspace-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.workspace-heading h2, .integration-heading h2 { margin: 7px 0; color: #102a43; font-size: 23px; }
.workspace-heading p, .integration-heading p { margin: 0; color: #64748b; line-height: 1.65; }
.blueprint-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.blueprint-block { padding: 16px; border: 1px solid #e2e8f0; border-radius: 14px; background: #f8fbff; }
.blueprint-block span, .blueprint-block strong { display: block; }
.blueprint-block span { color: #2563eb; font-size: 12px; font-weight: 800; }
.blueprint-block strong { margin-top: 8px; color: #0f2747; }
.blueprint-block p, .qa-placeholder p { margin: 8px 0 0; color: #64748b; line-height: 1.6; }
.qa-placeholder { display: grid; gap: 9px; max-width: 760px; margin-top: 18px; }
.qa-placeholder label { color: #334155; font-weight: 700; }
.activity-flow { display: grid; gap: 10px; margin-top: 18px; }
.flow-step { display: flex; gap: 12px; align-items: flex-start; padding: 13px; border: 1px solid #e2e8f0; border-radius: 13px; background: #f8fbff; }
.flow-step > span { display: grid; place-items: center; width: 26px; height: 26px; flex: 0 0 auto; border-radius: 50%; background: #dbeafe; color: #1d4ed8; font-weight: 800; }
.flow-step strong { color: #0f2747; }
.flow-step p { margin: 4px 0 0; color: #64748b; line-height: 1.55; }
.integration-card { display: grid; gap: 16px; }
.integration-heading h2 { margin-bottom: 5px; }
.checkpoint-grid { display: grid; grid-template-columns: repeat(4, minmax(0,1fr)); gap: 12px; }
.checkpoint-grid article { display: grid; align-content: start; gap: 7px; min-height: 130px; padding: 15px; border: 1px solid #e2e8f0; border-radius: 14px; background: #f8fbff; }
.checkpoint-grid article > span { color: #2563eb; font-size: 12px; font-weight: 900; }
.checkpoint-grid strong { color: #0f2747; }
.checkpoint-grid small { color: #64748b; line-height: 1.55; }
.approval-note { padding: 15px 17px; border-left: 4px solid #2563eb; border-radius: 0 12px 12px 0; background: #eff6ff; }
.approval-note strong { color: #1e3a8a; }
.approval-note p { margin: 5px 0 0; color: #475569; line-height: 1.6; }
@media (max-width: 900px) { .hero { align-items: flex-start; flex-direction: column; } .module-grid { grid-template-columns: 1fr; } .checkpoint-grid { grid-template-columns: repeat(2, minmax(0,1fr)); } }
@media (max-width: 600px) { .hero, .workspace-card, .integration-card { padding: 18px; } .status-card { min-width: 0; width: 100%; } .blueprint-grid, .checkpoint-grid { grid-template-columns: 1fr; } .workspace-heading { flex-direction: column; } }
</style>
