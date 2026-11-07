/**
 * The ONLY module that imports the generated Wails bindings (frontend/wailsjs,
 * gitignored; run `wails generate module` from the repo root to create it).
 * Everything else imports from '$lib/api' so tests can `vi.mock('$lib/api')`.
 * IsScanned and Startup are internal and intentionally not re-exported.
 */
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { app } from '../../wailsjs/go/models'

export {
  ListVolumes,
  IsElevated,
  RestartElevated,
  StartScan,
  CancelScan,
  GetSummary,
  GetChildren,
  ListFiles,
  GetTopFiles,
  GetExtensions,
  ListAIProfiles,
  SaveAIProfile,
  DeleteAIProfile,
  TestAIProfile,
  PreviewAIPayload,
  RunAI,
  CancelAI,
  GetAIReport,
  PlanClean,
  ExecuteClean,
  CancelClean,
  GetHistory,
  GetSettings,
  SaveSettings,
} from '../../wailsjs/go/app/App'

export type AIItemDTO = app.AIItemDTO
export type AIOpinionDTO = app.AIOpinionDTO
export type AIProfileDTO = app.AIProfileDTO
export type AIReportDTO = app.AIReportDTO
export type CategoryDTO = app.CategoryDTO
export type CleanupDTO = app.CleanupDTO
export type ExtDTO = app.ExtDTO
export type FileDTO = app.FileDTO
export type HistoryDTO = app.HistoryDTO
export type NodeDTO = app.NodeDTO
export type PayloadPreviewDTO = app.PayloadPreviewDTO
export type PlanDTO = app.PlanDTO
export type PlanItemDTO = app.PlanItemDTO
export type RefusalDTO = app.RefusalDTO
export type RuleProblemDTO = app.RuleProblemDTO
export type SettingsDTO = app.SettingsDTO
export type SummaryDTO = app.SummaryDTO
export type VolumeDTO = app.VolumeDTO

/** Subscribe to a backend event; returns the unsubscribe function. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function on(event: string, cb: (data: any) => void): () => void {
  return EventsOn(event, cb)
}
