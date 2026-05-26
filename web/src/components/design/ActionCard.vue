<template>
  <div class="ds-action-card" :class="[tone, { 'has-icon': icon }]">
    <div v-if="icon" class="ds-action-icon">{{ icon }}</div>
    <div class="ds-action-body">
      <span v-if="label">{{ label }}</span>
      <strong>{{ title }}</strong>
      <p v-if="description">{{ description }}</p>
    </div>
    <div v-if="$slots.action || $slots.actions" class="ds-action-actions">
      <slot name="action" />
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  icon: { type: String, default: '' },
  label: { type: String, default: '' },
  title: { type: String, required: true },
  description: { type: String, default: '' },
  tone: { type: String, default: 'blue' }
})
</script>

<style scoped>
.ds-action-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  border: 1px solid #dce8f5;
  border-radius: 16px;
  padding: 14px;
}

.ds-action-card.has-icon {
  grid-template-columns: 40px minmax(0, 1fr) auto;
}

.ds-action-card.blue {
  background: #eff6ff;
}

.ds-action-card.orange {
  background: #fff7ed;
}

.ds-action-card.green {
  background: #f0fdf4;
}

.ds-action-icon {
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: 14px;
  color: #fff;
  background: linear-gradient(135deg, #2563eb, #06b6d4);
  font-size: 22px;
  font-weight: 900;
}

span {
  display: block;
  color: #64748b;
  font-size: 12px;
  line-height: 1.4;
}

strong {
  display: block;
  margin: 3px 0 4px;
  color: #0f172a;
  font-size: 16px;
  line-height: 1.35;
}

p {
  margin: 0;
  color: #64748b;
  line-height: 1.55;
  font-size: 13px;
}

.ds-action-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 760px) {
  .ds-action-card {
    grid-template-columns: minmax(0, 1fr);
    align-items: stretch;
  }

  .ds-action-card.has-icon {
    grid-template-columns: 40px minmax(0, 1fr);
  }
}
</style>
