<template>
  <article class="ds-metric-card" :class="tone">
    <span>{{ label }}</span>
    <strong>{{ value }}</strong>
    <small v-if="hint">{{ hint }}</small>
  </article>
</template>

<script setup>
defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], required: true },
  hint: { type: String, default: '' },
  tone: { type: String, default: '' }
})
</script>

<style scoped>
.ds-metric-card {
  min-height: 104px;
  padding: 14px 16px;
  border: 1px solid #dce8f5;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 16px 42px rgba(15, 39, 71, 0.07);
}

.ds-metric-card.primary {
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
}

.ds-metric-card.cyan {
  background: linear-gradient(135deg, #ecfeff, #e0f2fe);
}

.ds-metric-card.success {
  background: linear-gradient(135deg, #f0fdf4, #dcfce7);
}

.ds-metric-card.warning {
  background: linear-gradient(135deg, #fff7ed, #ffedd5);
}

span {
  display: block;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}

strong {
  display: block;
  margin: 8px 0 6px;
  color: #0f172a;
  font-size: 26px;
  letter-spacing: -0.04em;
  line-height: 1.1;
}

small {
  display: block;
  color: #64748b;
  line-height: 1.5;
  font-size: 12px;
}
</style>
