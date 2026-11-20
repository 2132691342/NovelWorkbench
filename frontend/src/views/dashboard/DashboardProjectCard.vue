<!-- 仪表盘项目卡：封面、状态、指标与卡片内操作。 -->
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { NWProjectVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'

defineProps<{
  project: NWProjectVo
  genre_text: string
  status_text: string
  status_class: string
  cover_style: string
  relative_text: string
}>()

const emit = defineEmits<{
  open: []
  edit: []
  remove: []
}>()

const { t } = useI18n()
</script>

<template>
  <article class="project-card" @click="emit('open')">
    <div class="project-card__cover" :style="{ backgroundImage: cover_style }">
      <span class="project-card__cover-letter">{{ project.project_name.slice(0, 1) }}</span>
      <span class="project-card__cover-badge">{{ genre_text }}</span>
    </div>
    <div class="project-card__body">
      <div class="project-card__title-row">
        <h4 class="project-card__name" :title="project.project_name">{{ project.project_name }}</h4>
        <span class="nw-badge" :class="status_class">{{ status_text }}</span>
      </div>
      <p class="project-card__synopsis">
        {{ project.synopsis_text || t('dashboard.projectNoSynopsis') }}
      </p>
      <div class="project-card__metrics">
        <span><strong>{{ project.total_word_count.toLocaleString() }}</strong> {{ t('layout.wordsUnit') }}</span>
        <span><strong>{{ project.chapter_count }}</strong> {{ t('layout.chaptersUnit') }}</span>
        <span>{{ relative_text }}</span>
      </div>
      <div class="project-card__footer">
        <span class="project-card__open-hint">
          {{ t('dashboard.openWorkspace') }}
          <NWIcon icon_code="arrowRight" :size_px="13" />
        </span>
        <span class="project-card__card-actions">
          <el-tooltip :content="t('dashboard.editProject')" placement="top">
            <button type="button" class="project-card__icon-btn" @click.stop="emit('edit')">
              <NWIcon icon_code="edit3" :size_px="15" />
            </button>
          </el-tooltip>
          <el-tooltip :content="t('common.delete')" placement="top">
            <button type="button" class="project-card__icon-btn is-danger" @click.stop="emit('remove')">
              <NWIcon icon_code="trash" :size_px="15" />
            </button>
          </el-tooltip>
        </span>
      </div>
    </div>
  </article>
</template>

<style scoped>
.project-card {
  display: flex;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  overflow: hidden;
  cursor: pointer;
  transition: transform var(--transition-base), box-shadow var(--transition-base),
    border-color var(--transition-base);
}

.project-card:hover {
  transform: translateY(-4px);
  border-color: var(--border-accent);
  box-shadow: var(--shadow-card-hover);
}

.project-card__cover {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 88px;
  flex-shrink: 0;
  background-size: cover;
  background-position: center;
}

.project-card__cover-letter {
  color: rgba(255, 255, 255, 0.94);
  font-size: 34px;
  font-weight: 700;
  text-shadow: 0 2px 10px rgba(15, 23, 42, 0.25);
}

.project-card__cover-badge {
  position: absolute;
  left: 8px;
  bottom: 8px;
  padding: 2px 7px;
  border-radius: var(--radius-pill);
  background: rgba(15, 23, 42, 0.28);
  color: rgba(255, 255, 255, 0.92);
  font-size: 10px;
  backdrop-filter: blur(4px);
}

.project-card__body {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
  padding: 12px 14px 10px;
}

.project-card__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.project-card__name {
  margin: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-strong);
  font-size: 14.5px;
  font-weight: 650;
}

.project-card__synopsis {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin: 6px 0 8px;
  min-height: 36px;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.project-card__metrics {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--text-placeholder);
  font-size: 11px;
}

.project-card__metrics strong {
  color: var(--text-main);
  font-weight: 600;
}

.project-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
  padding-top: 10px;
}

.project-card__open-hint {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--primary);
  font-size: 11.5px;
  font-weight: 500;
  opacity: 0;
  transition: opacity var(--transition-base);
}

.project-card:hover .project-card__open-hint {
  opacity: 1;
}

.project-card__card-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-left: auto;
}

.project-card__icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--text-placeholder);
  cursor: pointer;
  transition: all var(--transition-base);
}

.project-card__icon-btn:hover {
  background: var(--primary-soft);
  color: var(--primary);
  transform: scale(1.05);
}

.project-card__icon-btn.is-danger:hover {
  background: var(--error-bg);
  color: var(--error);
}

.project-card__icon-btn:active {
  transform: scale(0.95);
}

@media (max-width: 760px) {
  .project-card__synopsis {
    -webkit-line-clamp: 3;
  }
}
</style>
