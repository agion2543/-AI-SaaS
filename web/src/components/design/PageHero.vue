<template>
  <section class="ds-page-hero" :class="{ compact }">
    <div>
      <div v-if="eyebrow" class="ds-eyebrow">{{ eyebrow }}</div>
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="ds-hero-actions">
      <slot name="actions" />
    </div>
  </section>
</template>

<script setup>
defineProps({
  eyebrow: { type: String, default: '' },
  title: { type: String, required: true },
  description: { type: String, default: '' },
  compact: { type: Boolean, default: false }
})
</script>

<style scoped>
.ds-page-hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
  min-height: 148px;
  padding: 24px;
  border: 1px solid rgba(147, 197, 253, 0.34);
  border-radius: 30px;
  color: #fff;
  background:
    radial-gradient(circle at 78% 18%, rgba(6, 182, 212, 0.42), transparent 30%),
    linear-gradient(135deg, #07182e 0%, #2563eb 58%, #06b6d4 100%);
  box-shadow: 0 24px 64px rgba(15, 39, 71, 0.16);
}

.ds-page-hero.compact {
  min-height: 112px;
  padding: 20px 24px;
}

.ds-eyebrow {
  color: #a7f3d0;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

h1 {
  margin: 8px 0;
  font-size: clamp(26px, 2.6vw, 36px);
  line-height: 1.08;
  letter-spacing: -0.05em;
}

p {
  max-width: 760px;
  margin: 0;
  color: rgba(255, 255, 255, 0.78);
  line-height: 1.65;
  font-size: 14px;
}

.ds-hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

@media (max-width: 760px) {
  .ds-page-hero {
    display: block;
  }

  .ds-hero-actions {
    margin-top: 16px;
  }
}
</style>
