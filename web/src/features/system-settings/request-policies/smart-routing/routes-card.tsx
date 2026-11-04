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
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'

import {
  parseSmartRoutes,
  routeEndpoints,
  serializeSmartRoutes,
  smartRoutesErrorMessage,
  SMART_TIER_IDS,
} from './lib'
import { RouteEditorDialog } from './route-editor-dialog'
import type { SmartRoute } from './types'

type SmartRoutesCardProps = {
  value: string
  onChange: (value: string) => void
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
  const { routes } = parseSmartRoutes(props.value)

  const changeMode = (next: string) => {
    if (next === 'json') {
      props.onChange(serializeSmartRoutes(routes, 2))
      setMode('json')
      return
    }
    const { error } = parseSmartRoutes(props.value)
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
              disabled={mode === 'json'}
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
          onOpenChange={(open) => !open && setEditorTarget(null)}
          onSave={(route) =>
            props.onChange(
              serializeSmartRoutes(
                editorTarget.route === null
                  ? [...routes, route]
                  : routes.map((item, index) =>
                      index === editorTarget.index ? route : item
                    )
              )
            )
          }
        />
      )}

      {deleting !== null && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setDeleting(null)}
          title={t('Delete virtual model')}
          desc={t('Delete {{name}}? It takes effect after you save changes.', {
            name: deleting.name,
          })}
          confirmText={t('Delete')}
          handleConfirm={() => {
            props.onChange(
              serializeSmartRoutes(
                routes.filter((_, index) => index !== deleting.index)
              )
            )
            setDeleting(null)
          }}
          destructive
        />
      )}
    </>
  )
}

type SmartRoutesTableProps = {
  routes: SmartRoute[]
  onAdd: () => void
  onEdit: (route: SmartRoute) => void
  onDelete: (route: SmartRoute) => void
}

function SmartRoutesTable(props: SmartRoutesTableProps) {
  const { t } = useTranslation()
  const { models } = usePricingData()
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
          <Button variant='outline' onClick={props.onAdd}>
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
      tableClassName='min-w-[1080px] table-fixed [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4 [&_code]:font-mono!'
      headerRowClassName='bg-muted/35 hover:bg-muted/35'
      data={props.routes}
      columns={[
        {
          id: 'name',
          header: t('Name'),
          className: 'w-[16%]',
          cell: (route) => {
            const endpoints = routeEndpoints(route, models)
            return (
              <div className='flex min-w-0 flex-col gap-1.5'>
                <TruncatedCell tabIndex={0}>{route.name}</TruncatedCell>
                {endpoints.length > 0 ? (
                  <div
                    className='text-muted-foreground min-w-0'
                    data-table-text='secondary'
                    aria-label={t('Endpoints')}
                  >
                    <TruncatedCell tabIndex={0}>
                      {t('{{endpoint}} endpoint', {
                        endpoint: endpoints.join(' / '),
                        interpolation: { escapeValue: false },
                      })}
                    </TruncatedCell>
                  </div>
                ) : null}
              </div>
            )
          },
        },
        ...SMART_TIER_IDS.map((id) => ({
          id,
          header: tierTitles[id],
          cell: (route: SmartRoute) => (
            <div className='flex min-w-0 flex-col gap-1.5'>
              <div className='flex min-w-0 items-center gap-2'>
                <TruncatedCell tabIndex={0}>
                  {route.tiers[id].model}
                </TruncatedCell>
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
                <TruncatedCell tabIndex={0}>
                  <code>
                    {route.tiers[id].groups.join(' → ')} ·{' '}
                    {route.tiers[id].effort || t('Keep client value')}
                  </code>
                </TruncatedCell>
              </div>
            </div>
          ),
        })),
        {
          id: 'scorer',
          header: t('Scorer model'),
          cell: (route) => (
            <div className='flex min-w-0 flex-col gap-1.5'>
              <TruncatedCell tabIndex={0}>{route.scorer.model}</TruncatedCell>
              <div
                className='text-muted-foreground min-w-0'
                data-table-text='secondary'
                aria-label={t('Groups')}
              >
                <TruncatedCell tabIndex={0}>
                  <code>
                    {route.scorer.groups.join(' → ')} ·{' '}
                    {t('Timeout {{ms}} ms', { ms: route.scorer.timeout_ms })}
                  </code>
                </TruncatedCell>
              </div>
            </div>
          ),
        },
        {
          id: 'advisor',
          header: t('Advisor'),
          cell: (route) => (
            <div className='flex min-w-0 flex-col gap-1.5'>
              <TruncatedCell tabIndex={0}>{route.advisor.model}</TruncatedCell>
              <div
                className='text-muted-foreground min-w-0'
                data-table-text='secondary'
              >
                <TruncatedCell tabIndex={0}>
                  {t('After {{count}} dissatisfied messages', {
                    count: route.advisor.after_dissatisfied,
                  })}
                </TruncatedCell>
              </div>
            </div>
          ),
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
