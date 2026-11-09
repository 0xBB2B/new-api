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
import { Braces, Edit, ListFilter, Plus, Route, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DataTableRowActionMenu,
  StaticDataTable,
  TruncatedCell,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuGroup,
  DropdownMenuItem,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import type { BuiltinThinkingCapabilities } from './api'
import { effectiveCapability, effortOverflow } from './capability'
import {
  endpointShortNames,
  hasMissingEndpoints,
  parseSmartRoutes,
  serializeSmartRoutes,
  smartRoutesErrorMessage,
  SMART_TIER_IDS,
  tierEffortSummary,
} from './lib'
import { RouteEditorDialog } from './route-editor-dialog'
import { normalizeTier } from './route-form'
import type { SmartRoute, ThinkingCapabilityOverride } from './types'

type SmartRoutesCardProps = {
  value: string
  onChange: (value: string) => void
  onSave: (routes: SmartRoute[]) => Promise<void>
  saving: boolean
  overrides: Record<string, ThinkingCapabilityOverride>
  builtin: BuiltinThinkingCapabilities | undefined
}

export function SmartRoutesCard(props: SmartRoutesCardProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [deleting, setDeleting] = useState<{
    index: number
    name: string
  } | null>(null)
  const [editorTarget, setEditorTarget] = useState<{
    route: SmartRoute | null
    index: number
  } | null>(null)
  const [deletingPending, setDeletingPending] = useState(false)
  const { routes } = parseSmartRoutes(props.value)

  const changeMode = (next: string) => {
    if (next === 'json') {
      props.onChange(serializeSmartRoutes(routes, 2))
      setMode('json')
      return
    }
    const parsed = parseSmartRoutes(props.value)
    const error =
      parsed.error ?? (hasMissingEndpoints(parsed.routes) ? 'incomplete' : null)
    if (error) {
      toast.error(smartRoutesErrorMessage(t, error))
      return
    }
    setMode('visual')
  }

  return (
    <>
      <section
        aria-label={t('Virtual models')}
        className='bg-card min-w-0 overflow-hidden rounded-xl border'
      >
        <div className='flex flex-col gap-1.5 px-4 pt-5 sm:px-5'>
          <div className='flex items-center gap-2'>
            <h3 className='text-base font-semibold'>{t('Virtual models')}</h3>
            <Badge variant='secondary' className='tabular-nums'>
              {routes.length}
            </Badge>
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Each virtual model has three tier models, a scorer model and an advisor. Users enter the virtual model name as the model in their client.'
            )}
          </p>
        </div>
        <Tabs value={mode} onValueChange={changeMode} className='gap-0'>
          <div className='flex flex-wrap items-center justify-between gap-3 px-4 py-4 sm:px-5'>
            <TabsList aria-label={t('Virtual model editor mode')}>
              <TabsTrigger value='visual' className='px-3'>
                <ListFilter aria-hidden='true' />
                {t('Visual')}
              </TabsTrigger>
              <TabsTrigger value='json' className='px-3'>
                <Braces aria-hidden='true' />
                JSON
              </TabsTrigger>
            </TabsList>
            <Button
              disabled={mode === 'json' || props.saving}
              onClick={() => setEditorTarget({ route: null, index: -1 })}
            >
              <Plus aria-hidden='true' data-icon='inline-start' />
              {t('Add virtual model')}
            </Button>
          </div>
          <Separator />
          <TabsContent value='visual' className='min-w-0'>
            <SmartRoutesTable
              routes={routes}
              overrides={props.overrides}
              builtin={props.builtin}
              saving={props.saving}
              onAdd={() => setEditorTarget({ route: null, index: -1 })}
              onEdit={(route) =>
                setEditorTarget({ route, index: routes.indexOf(route) })
              }
              onDelete={(route) =>
                setDeleting({ index: routes.indexOf(route), name: route.name })
              }
            />
          </TabsContent>
          <TabsContent value='json' className='min-w-0'>
            <div className='p-4 sm:p-5'>
              <JsonCodeEditor
                ariaLabel={t('Smart routing')}
                value={props.value}
                onChange={props.onChange}
                heightClassName='h-[360px] min-h-[360px] max-h-[360px]'
              />
            </div>
          </TabsContent>
        </Tabs>
      </section>

      {editorTarget !== null && (
        <RouteEditorDialog
          route={editorTarget.route}
          overrides={props.overrides}
          builtin={props.builtin}
          otherNames={routes
            .filter((_, index) => index !== editorTarget.index)
            .map((item) => item.name)}
          onOpenChange={(open) => !open && setEditorTarget(null)}
          onSave={(route) =>
            props.onSave(
              editorTarget.route === null
                ? [...routes, route]
                : routes.map((item, index) =>
                    index === editorTarget.index ? route : item
                  )
            )
          }
        />
      )}

      {deleting !== null && (
        <ConfirmDialog
          open
          onOpenChange={(open) =>
            !open && !deletingPending && setDeleting(null)
          }
          title={t('Delete virtual model')}
          desc={t(
            'Delete {{name}}? It takes effect immediately after you confirm.',
            { name: deleting.name }
          )}
          confirmText={deletingPending ? t('Saving...') : t('Delete')}
          isLoading={deletingPending}
          handleConfirm={async () => {
            setDeletingPending(true)
            try {
              await props.onSave(
                routes.filter((_, index) => index !== deleting.index)
              )
              setDeleting(null)
            } catch {
              // 错误提示已由保存请求统一弹出
            } finally {
              setDeletingPending(false)
            }
          }}
          destructive
        />
      )}
    </>
  )
}

type SmartRoutesTableProps = {
  routes: SmartRoute[]
  overrides: Record<string, ThinkingCapabilityOverride>
  builtin: BuiltinThinkingCapabilities | undefined
  saving: boolean
  onAdd: () => void
  onEdit: (route: SmartRoute) => void
  onDelete: (route: SmartRoute) => void
}

function SmartRoutesTable(props: SmartRoutesTableProps) {
  const { t } = useTranslation()
  const builtin = props.builtin ?? {}
  const tierTitles = {
    simple: t('Simple tier'),
    normal: t('Normal tier'),
    hard: t('Hard tier'),
  }

  if (props.routes.length === 0) {
    return (
      <EmptyState
        icon={Route}
        title={t('No virtual models yet')}
        description={t(
          'Add a virtual model such as auto-gpt. Users enter it as the model name in their client.'
        )}
        className='min-h-56'
        action={
          <Button
            variant='outline'
            disabled={props.saving}
            onClick={props.onAdd}
          >
            <Plus aria-hidden='true' data-icon='inline-start' />
            {t('Add virtual model')}
          </Button>
        }
      />
    )
  }

  return (
    <StaticDataTable
      className='focus-visible:outline-ring overflow-x-auto rounded-none border-0 focus-visible:-outline-offset-2'
      containerProps={{
        role: 'region',
        'aria-label': t('Virtual models'),
        tabIndex: 0,
      }}
      tableProps={{ withContainer: false }}
      tableClassName='min-w-[960px] table-fixed [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4 [&_code]:font-mono!'
      headerRowClassName='bg-muted/35 hover:bg-muted/35'
      data={props.routes}
      columns={[
        {
          id: 'name',
          header: t('Name'),
          className: 'w-[16%]',
          cell: (route) => {
            const endpointText = t('{{endpoint}} endpoint', {
              endpoint: endpointShortNames(route.endpoints ?? []),
              interpolation: { escapeValue: false },
            })
            return (
              <div className='flex min-w-0 flex-col gap-1.5'>
                <TruncatedCell tabIndex={0}>{route.name}</TruncatedCell>
                <div
                  className='text-muted-foreground min-w-0'
                  data-table-text='secondary'
                  aria-label={t('Endpoints')}
                >
                  <TruncatedCell tabIndex={0} tooltipContent={endpointText}>
                    <code>{endpointText}</code>
                  </TruncatedCell>
                </div>
              </div>
            )
          },
        },
        ...SMART_TIER_IDS.map((id) => ({
          id,
          header: tierTitles[id],
          cell: (route: SmartRoute) => {
            const tier = normalizeTier(route.tiers[id])
            const known = tier.model in builtin || !!props.overrides[tier.model]
            const capability = effectiveCapability(
              tier.model,
              builtin,
              props.overrides
            )
            const overflow = known
              ? effortOverflow(tier, capability)
              : { overflow: false, actual: '' }
            const overflowText = t(
              'Exceeds the model capability, sent as {{effort}}',
              { effort: overflow.actual }
            )
            const summary = tierEffortSummary(tier, capability, t)
            const lineText = `${summary} · ${tier.groups.join(' → ')}`
            return (
              <div className='flex min-w-0 flex-col gap-1.5'>
                <div className='flex min-w-0 items-center gap-2'>
                  <TruncatedCell tabIndex={0}>{tier.model}</TruncatedCell>
                  {route.default_tier === id ? (
                    <Badge variant='outline' className='shrink-0'>
                      {t('Default')}
                    </Badge>
                  ) : null}
                </div>
                <div
                  className='text-muted-foreground min-w-0'
                  data-table-text='secondary'
                  aria-label={t('Groups')}
                >
                  <TruncatedCell tabIndex={0} tooltipContent={lineText}>
                    <code>
                      <span className={overflow.overflow ? 'text-warning' : ''}>
                        {summary}
                      </span>{' '}
                      · {tier.groups.join(' → ')}
                    </code>
                  </TruncatedCell>
                </div>
                {overflow.overflow ? (
                  <div
                    className='text-warning min-w-0'
                    data-table-text='secondary'
                  >
                    <TruncatedCell tabIndex={0}>{overflowText}</TruncatedCell>
                  </div>
                ) : null}
              </div>
            )
          },
        })),
        {
          id: 'scorer',
          header: t('Scorer model'),
          cell: (route) => {
            const scorerText = `${route.scorer.groups.join(' → ')} · ${t('Timeout {{ms}} ms', { ms: route.scorer.timeout_ms })}`
            return (
              <div className='flex min-w-0 flex-col gap-1.5'>
                <TruncatedCell tabIndex={0}>{route.scorer.model}</TruncatedCell>
                <div
                  className='text-muted-foreground min-w-0'
                  data-table-text='secondary'
                  aria-label={t('Groups')}
                >
                  <TruncatedCell tabIndex={0} tooltipContent={scorerText}>
                    <code>{scorerText}</code>
                  </TruncatedCell>
                </div>
              </div>
            )
          },
        },
        {
          id: 'advisor',
          header: t('Advisor'),
          cell: (route) => {
            const advisorText = t('After {{count}} dissatisfied messages', {
              count: route.advisor.after_dissatisfied,
            })
            return (
              <div className='flex min-w-0 flex-col gap-1.5'>
                <TruncatedCell tabIndex={0}>
                  {route.advisor.model}
                </TruncatedCell>
                <div
                  className='text-muted-foreground min-w-0'
                  data-table-text='secondary'
                >
                  <TruncatedCell tabIndex={0} tooltipContent={advisorText}>
                    <code>{advisorText}</code>
                  </TruncatedCell>
                </div>
              </div>
            )
          },
        },
        {
          id: 'actions',
          header: t('Actions'),
          className: 'w-24 text-right',
          cell: (route) => (
            <div
              role='group'
              aria-label={t('Actions for {{name}}', { name: route.name })}
              className='flex items-center justify-end gap-0.5'
            >
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Edit')}
                      disabled={props.saving}
                      onClick={() => props.onEdit(route)}
                    >
                      <Edit aria-hidden='true' />
                    </Button>
                  }
                />
                <TooltipContent>{t('Edit')}</TooltipContent>
              </Tooltip>
              <DataTableRowActionMenu ariaLabel={t('More actions')}>
                <DropdownMenuGroup>
                  <DropdownMenuItem
                    variant='destructive'
                    disabled={props.saving}
                    onClick={() => props.onDelete(route)}
                  >
                    <Trash2 aria-hidden='true' />
                    {t('Delete')}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DataTableRowActionMenu>
            </div>
          ),
        },
      ]}
    />
  )
}
