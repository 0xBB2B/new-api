/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { AlertTriangle, Download } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { LoadingState } from '@/components/loading-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { invalidateModelPricing } from '@/features/model-pricing/api'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import {
  getServerErrorMessage,
  getServerErrorStatus,
  requireServerSuccess,
  translateMessageParams,
} from '@/lib/server-error-message'

import {
  getModelCapabilities,
  previewModelCapabilities,
  saveModelCapabilities,
} from '../../api'
import { getNameRuleOptions } from '../../constants'
import { formatModelCapabilityJson } from '../../lib/model-capabilities'
import type {
  CapabilitiesMap,
  CapabilitiesPreviewAction,
  CapabilitiesPreviewItem as PreviewItem,
  PreviewModelCapabilitiesResponse,
} from '../../types'

const STEPS = ['1. Review and edit', '2. Preview and save']
const SAMPLE_CAPABILITY = {
  context_length: 1050000,
  top_provider: { max_completion_tokens: 128000 },
  architecture: {
    input_modalities: ['text', 'image'],
    output_modalities: ['text'],
  },
  reasoning: { supported_efforts: ['low', 'medium', 'high'] },
}
const FIELD_LABELS: Record<string, string> = {
  context_length: 'Context length',
  'top_provider.max_completion_tokens': 'Max output tokens',
  'architecture.input_modalities': 'Input modalities',
  'architecture.output_modalities': 'Output modalities',
  'reasoning.supported_efforts': 'Thinking levels',
}
const ACTION_ORDER: CapabilitiesPreviewAction[] = [
  'create',
  'update',
  'clear',
  'unchanged',
]
const ACTION_LABELS: Record<CapabilitiesPreviewAction, string> = {
  create: 'Create',
  update: 'Update',
  clear: 'Clear',
  unchanged: 'Unchanged',
}
const ACTION_VARIANTS = {
  create: 'secondary',
  update: 'secondary',
  clear: 'destructive',
  unchanged: 'outline',
} as const

type PreviewState = {
  capabilities: CapabilitiesMap
  result: PreviewModelCapabilitiesResponse['data']
}
type SaveIssue = { stale: boolean; message: string }

function parseCapabilities(text: string): CapabilitiesMap | null {
  try {
    const value: unknown = JSON.parse(text.trim() === '' ? '{}' : text)
    if (value === null || typeof value !== 'object' || Array.isArray(value)) {
      return null
    }
    return value as CapabilitiesMap
  } catch {
    return null
  }
}

function flattenFields(
  value: Record<string, unknown> | null,
  prefix = ''
): Map<string, unknown> {
  const fields = new Map<string, unknown>()
  for (const [key, child] of Object.entries(value ?? {})) {
    const path = prefix + key
    if (child !== null && typeof child === 'object' && !Array.isArray(child)) {
      for (const [nested, nestedValue] of flattenFields(
        child as Record<string, unknown>,
        `${path}.`
      )) {
        fields.set(nested, nestedValue)
      }
    } else if (child !== null && child !== undefined) {
      fields.set(path, child)
    }
  }
  return fields
}

type Translate = (key: string, options?: Record<string, unknown>) => string

function formatFieldValue(
  path: string,
  value: unknown,
  t: Translate,
  locale: string | undefined
): string {
  if (Array.isArray(value)) {
    if (value.length > 0) return value.join(', ')
    return path === 'reasoning.supported_efforts'
      ? t('No thinking support')
      : t('None')
  }
  if (typeof value === 'number') return formatNumber(value, locale)
  return String(value)
}

export function CapabilityPreviewItem(props: { item: PreviewItem }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const item = props.item
  const before = flattenFields(item.before)
  const after = flattenFields(item.after)
  const changed = [...new Set([...before.keys(), ...after.keys()])].filter(
    (path) =>
      JSON.stringify(before.get(path)) !== JSON.stringify(after.get(path))
  )

  let body = null
  if (item.action === 'unchanged') {
    body = (
      <p className='text-muted-foreground text-sm'>
        {t('Same as the current configuration. Nothing will change.')}
      </p>
    )
  } else {
    body = (
      <>
        <StaticDataTable
          columns={[
            {
              id: 'field',
              header: t('Field'),
              cell: (path: string) =>
                FIELD_LABELS[path] ? t(FIELD_LABELS[path]) : path,
            },
            {
              id: 'before',
              header: t('Current value'),
              cell: (path: string) =>
                before.has(path)
                  ? formatFieldValue(path, before.get(path), t, locale)
                  : '—',
            },
            {
              id: 'after',
              header: t('Change to'),
              cell: (path: string) =>
                after.has(path)
                  ? formatFieldValue(path, after.get(path), t, locale)
                  : t('Cleared (not configured)'),
            },
          ]}
          data={changed}
          getRowKey={(path) => path}
        />
        {item.action === 'create' && (
          <p className='text-muted-foreground text-sm'>
            {t(
              'This model has no record in Model Management yet. An exact-match record will be created.'
            )}
          </p>
        )}
        {item.action === 'clear' && (
          <p className='text-muted-foreground text-sm'>
            {t(
              'The model is not in the JSON, so its capabilities will be cleared. The record itself is kept.'
            )}
          </p>
        )}
      </>
    )
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-center gap-2'>
        <span className='font-mono text-sm font-medium'>{item.model_name}</span>
        <Badge variant={ACTION_VARIANTS[item.action]}>
          {t(ACTION_LABELS[item.action])}
        </Badge>
      </div>
      {body}
    </div>
  )
}

function CapabilitiesJsonDialogContent(props: {
  onOpenChange: (open: boolean) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const nameRules = getNameRuleOptions(t)
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<string | null>(null)
  const [preview, setPreview] = useState<PreviewState | null>(null)
  const [saveIssue, setSaveIssue] = useState<SaveIssue | null>(null)

  const query = useQuery({
    queryKey: ['models', 'capabilities-json'],
    queryFn: async () =>
      requireServerSuccess(await getModelCapabilities()).data,
    gcTime: 0,
    refetchOnWindowFocus: false,
  })

  const previewMutation = useMutation({
    mutationFn: async (capabilities: CapabilitiesMap) => ({
      capabilities,
      result: requireServerSuccess(await previewModelCapabilities(capabilities))
        .data,
    }),
    onSuccess: (state) => {
      setSaveIssue(null)
      setPreview(state)
    },
    onError: (error) => handleServerError(error),
  })

  const saveMutation = useMutation({
    mutationFn: (state: PreviewState) =>
      saveModelCapabilities(state.capabilities, state.result.version),
    onSuccess: async (response) => {
      const saved = requireServerSuccess(response).data
      toast.success(t('Model capabilities saved'), {
        description: t(
          'Created {{create}}, updated {{update}}, cleared {{clear}} records',
          saved
        ),
      })
      await invalidateModelPricing(queryClient)
      props.onOpenChange(false)
    },
    onError: (error) =>
      setSaveIssue({
        stale: getServerErrorStatus(error) === 409,
        message: getServerErrorMessage(error),
      }),
  })

  const data = query.data
  const modelCount = Object.keys(data?.capabilities ?? {}).length
  const baseline = modelCount
    ? formatModelCapabilityJson(data?.capabilities)
    : ''
  const text = draft ?? baseline
  const dirty = text !== baseline
  const parsed = parseCapabilities(text)
  const step = preview ? 1 : 0

  const handleDownload = () => {
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `model-capabilities-${dayjs().format('YYYY-MM-DD')}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  const backToEditing = () => {
    setPreview(null)
    setSaveIssue(null)
  }

  const result = preview?.result
  const errorCount = result?.errors.length ?? 0
  const changeCount = result
    ? result.summary.create + result.summary.update + result.summary.clear
    : 0
  const failedGroups = new Map<string, string[]>()
  for (const error of result?.errors ?? []) {
    const message = t(
      error.message.key,
      error.message.params && translateMessageParams(error.message.params)
    )
    failedGroups.set(error.model_name, [
      ...(failedGroups.get(error.model_name) ?? []),
      message,
    ])
  }
  const submittedOrder = Object.keys(preview?.capabilities ?? {})
  const submittedPosition = (name: string) => {
    const position = submittedOrder.indexOf(name)
    return position === -1 ? Number.MAX_SAFE_INTEGER : position
  }
  const failedRows = [...failedGroups]
    .map(([model, problems]) => ({ model, problems }))
    .sort(
      (a, b) =>
        submittedPosition(a.model) - submittedPosition(b.model) ||
        a.model.localeCompare(b.model)
    )
  const orderedItems = [...(result?.items ?? [])].sort(
    (a, b) =>
      ACTION_ORDER.indexOf(a.action) - ACTION_ORDER.indexOf(b.action) ||
      a.model_name.localeCompare(b.model_name)
  )
  const cleared = result?.items.filter((item) => item.action === 'clear') ?? []
  const n = (value: number | undefined) => formatNumber(value, locale)
  const summaryLabels = {
    create: t('Create {{count}}', { count: n(result?.summary.create) }),
    update: t('Update {{count}}', { count: n(result?.summary.update) }),
    clear: t('Clear {{count}}', { count: n(result?.summary.clear) }),
    unchanged: t('Unchanged {{count}}', {
      count: n(result?.summary.unchanged),
    }),
  }
  const saving = saveMutation.isPending
  const stale = saveIssue?.stale === true

  const unconfigured = modelCount === 0 ? [] : (data?.unconfigured ?? [])
  let footer = null
  if (step === 0) {
    footer = (
      <>
        <div className='text-muted-foreground mr-auto min-w-0 text-sm'>
          {data && modelCount === 0 && !dirty && (
            <p>
              {t(
                'No model has capabilities configured yet. You can paste JSON directly.'
              )}
            </p>
          )}
          {data && modelCount > 0 && !dirty && (
            <p>
              {t('{{count}} models have capabilities configured', {
                count: n(modelCount),
              })}
            </p>
          )}
          {dirty && <p>{t('You have unsaved changes')}</p>}
        </div>
        <Button variant='outline' onClick={() => props.onOpenChange(false)}>
          {t('Close')}
        </Button>
        <Button
          variant='outline'
          disabled={!text.trim()}
          onClick={handleDownload}
        >
          <Download className='h-4 w-4' aria-hidden='true' />
          {t('Download JSON file')}
        </Button>
        <Button
          disabled={!dirty || !parsed || previewMutation.isPending}
          onClick={() => parsed && previewMutation.mutate(parsed)}
        >
          {t('Preview changes')}
        </Button>
      </>
    )
  } else if (result && preview) {
    footer = (
      <>
        {errorCount > 0 && (
          <span className='text-muted-foreground mr-auto text-sm'>
            {t('{{count}} entries failed validation', {
              count: n(failedRows.length),
            })}
          </span>
        )}
        {errorCount === 0 && (
          <span className='text-muted-foreground mr-auto text-sm'>
            {t(
              'Will create {{create}}, update {{update}}, and clear {{clear}} records',
              {
                create: n(result.summary.create),
                update: n(result.summary.update),
                clear: n(result.summary.clear),
              }
            )}
          </span>
        )}
        <Button variant='outline' disabled={saving} onClick={backToEditing}>
          {t('Back to editing')}
        </Button>
        {stale ? (
          <Button
            disabled={previewMutation.isPending}
            onClick={() => previewMutation.mutate(preview.capabilities)}
          >
            {t('Preview again')}
          </Button>
        ) : (
          <Button
            disabled={saving || errorCount > 0 || changeCount === 0}
            onClick={() => saveMutation.mutate(preview)}
          >
            {saving
              ? t('Saving...')
              : t('Replace and save {{count}} changes', {
                  count: n(changeCount),
                })}
          </Button>
        )}
      </>
    )
  }

  let content = null
  if (query.isLoading) {
    content = <LoadingState />
  } else if (query.isError || !data) {
    content = (
      <ErrorState
        description={t('Failed to load model capabilities')}
        onRetry={() => query.refetch()}
      />
    )
  } else if (step === 0) {
    const notListed = data.non_exact.length + unconfigured.length
    content = (
      <div className='space-y-4'>
        {notListed > 0 && (
          <Alert>
            <AlertTriangle aria-hidden='true' />
            <AlertTitle>
              {t('{{count}} records are not listed below', {
                count: n(notListed),
              })}
            </AlertTitle>
            <AlertDescription className='space-y-2'>
              {data.non_exact.length > 0 && (
                <p>
                  {t(
                    '{{count}} records are not exact-match records: {{names}}.',
                    {
                      count: n(data.non_exact.length),
                      names: data.non_exact
                        .map((entry) =>
                          t('{{name}} ({{rule}})', {
                            name: entry.model_name,
                            rule: nameRules.find(
                              (rule) => rule.value === entry.name_rule
                            )?.label,
                          })
                        )
                        .join(t('、')),
                    }
                  )}{' '}
                  {t(
                    'They are not specific model names. Edit them in the model drawer; saving here will not touch them.'
                  )}
                </p>
              )}
              {unconfigured.length > 0 && (
                <p>
                  {t(
                    '{{count}} records have no capabilities configured yet: {{names}}.',
                    {
                      count: n(unconfigured.length),
                      names: unconfigured.join(t('、')),
                    }
                  )}{' '}
                  {unconfigured.length === 1
                    ? t('To configure it, add its name below.')
                    : t('To configure them, add their names below.')}
                </p>
              )}
            </AlertDescription>
          </Alert>
        )}
        <div className='space-y-2'>
          <Label>{t('Capability configuration (JSON)')}</Label>
          <JsonCodeEditor
            value={text}
            onChange={setDraft}
            placeholder={formatModelCapabilityJson({
              [t('Model name')]: SAMPLE_CAPABILITY,
            })}
            ariaLabel={t('Capabilities JSON')}
            heightClassName='h-80 min-h-80 max-h-80'
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              "Saving replaces everything. After saving, each model's capabilities will be exactly what is written here. Fields that are omitted or set to null will be cleared. Removing a model from the JSON clears its capabilities but keeps the record. Models without a record in Model Management get a new exact-match record automatically."
            )}
          </p>
          {!parsed && (
            <p className='text-destructive text-sm'>
              {t('The JSON is malformed. Fix it before previewing.')}
            </p>
          )}
        </div>
      </div>
    )
  } else if (result) {
    content = (
      <div className='space-y-4'>
        <div className='flex flex-wrap gap-2'>
          {(['create', 'update', 'clear', 'unchanged'] as const).map(
            (action) =>
              result.summary[action] > 0 && (
                <Badge key={action} variant={ACTION_VARIANTS[action]}>
                  {summaryLabels[action]}
                </Badge>
              )
          )}
          {errorCount > 0 && (
            <Badge variant='destructive'>
              {t('Validation failed {{count}}', {
                count: n(failedRows.length),
              })}
            </Badge>
          )}
        </div>
        {saveIssue && (
          <Alert variant='destructive'>
            <AlertTriangle aria-hidden='true' />
            <AlertTitle>
              {stale
                ? t('Model data has changed. Nothing was saved.')
                : t('Save failed. Nothing was changed.')}
            </AlertTitle>
            <AlertDescription>
              {stale
                ? t(
                    'After the preview, some model data was changed, so the comparison below is out of date. Preview again, confirm the latest changes, then save.'
                  )
                : `${saveIssue.message} ${t('You can click save again later.')}`}
            </AlertDescription>
          </Alert>
        )}
        {errorCount > 0 && (
          <>
            <Alert variant='destructive'>
              <AlertTriangle aria-hidden='true' />
              <AlertTitle>
                {t(
                  '{{count}} entries failed validation and cannot be saved now',
                  {
                    count: n(failedRows.length),
                  }
                )}
              </AlertTitle>
              <AlertDescription>
                {t(
                  'Go back and fix the JSON. You can save once everything passes.'
                )}
              </AlertDescription>
            </Alert>
            <div className='space-y-2'>
              <h3 className='text-sm font-medium'>
                {t('Entries that failed validation ({{count}})', {
                  count: n(failedRows.length),
                })}
              </h3>
              <StaticDataTable
                columns={[
                  {
                    id: 'model',
                    header: t('Model'),
                    cell: (row) => (
                      <span className='font-mono text-xs'>{row.model}</span>
                    ),
                  },
                  {
                    id: 'problem',
                    header: t('Problem'),
                    cellClassName: 'whitespace-normal break-words',
                    cell: (row) => (
                      <div className='space-y-1'>
                        {row.problems.map((problem) => (
                          <div key={problem}>{problem}</div>
                        ))}
                      </div>
                    ),
                  },
                ]}
                data={failedRows}
                getRowKey={(row) => row.model}
              />
            </div>
          </>
        )}
        {cleared.length > 0 && (
          <Alert>
            <AlertTriangle aria-hidden='true' />
            <AlertTitle>
              {t('Capabilities of {{count}} models will be cleared', {
                count: n(cleared.length),
              })}
            </AlertTitle>
            <AlertDescription>
              <ul className='list-disc ps-4'>
                {cleared.map((item) => (
                  <li key={item.model_name}>
                    {t(
                      '{{model}} is not in the JSON. If you did not mean to remove it, go back and add it again.',
                      { model: item.model_name }
                    )}
                  </li>
                ))}
              </ul>
            </AlertDescription>
          </Alert>
        )}
        <div className='space-y-3'>
          {orderedItems.map((item) => (
            <CapabilityPreviewItem key={item.model_name} item={item} />
          ))}
        </div>
      </div>
    )
  }

  return (
    <Dialog
      open
      onOpenChange={(value) => props.onOpenChange(value)}
      title={t('Capabilities JSON')}
      description={t(
        'These are the current capabilities of all models, organized by model name, in the same format as /v1/models. You can copy or download them, edit them directly, or paste new content and save.'
      )}
      contentClassName='sm:max-w-3xl'
      contentHeight='min(75vh, 800px)'
      footer={footer}
    >
      {data && !query.isError && (
        <ol className='mb-5 grid grid-cols-2 gap-2'>
          {STEPS.map((label, index) => (
            <li
              key={label}
              aria-current={step === index ? 'step' : undefined}
              className={
                step === index
                  ? 'border-primary bg-primary/5 rounded-lg border p-2 text-sm font-medium'
                  : 'text-muted-foreground rounded-lg border p-2 text-sm'
              }
            >
              {t(label)}
            </li>
          ))}
        </ol>
      )}
      {content}
    </Dialog>
  )
}

export function CapabilitiesJsonDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  if (!props.open) return null
  return <CapabilitiesJsonDialogContent onOpenChange={props.onOpenChange} />
}
