<template>
  <section class="ds-data-panel">
    <div v-if="title || eyebrow || $slots.actions" class="ds-panel-head">
      <div>
        <div v-if="eyebrow" class="ds-eyebrow">{{ eyebrow }}</div>
        <h2 v-if="title">{{ title }}</h2>
        <p v-if="description">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="ds-panel-actions">
        <slot name="actions" />
      </div>
    </div>
    <slot />
  </section>
</template>

<script setup>
defineProps({
  eyebrow: { type: String, default: '' },
  title: { type: String, default: '' },
  description: { type: String, default: '' }
})
</script>

<style scoped>
.ds-data-panel {
  padding: 18px;
  border: 1px solid #dce8f5;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 18px 46px rgba(15, 39, 71, 0.07);
}

.ds-panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.ds-eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

h2 {
  margin: 4px 0 6px;
  color: #0f172a;
  font-size: 22px;
  letter-spacing: -0.03em;
}

p {
  margin: 0;
  color: #64748b;
  line-height: 1.6;
  font-size: 13px;
}

.ds-panel-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

@media (max-width: 760px) {
  .ds-panel-head {
    display: block;
  }

  .ds-panel-actions {
    margin-top: 12px;
  }
}
</style>
