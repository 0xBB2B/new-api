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
import { Brain } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable, TruncatedCell } from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

import type { BuiltinThinkingCapabilities } from './api'
import { effectiveCapability, tierModelsOfRoutes } from './capability'
import { CapabilityOverrideDialog } from './capability-override-dialog'
import type { SmartRoute, ThinkingCapabilityOverride } from './types'

type ThinkingCapabilitiesCardProps = {
  routes: SmartRoute[]
  builtin: BuiltinThinkingCapabilities | undefined
  isPending: boolean
  isError: boolean
  onRetry: () => void
  overrides: Record<string, ThinkingCapabilityOverride>
  onChange: (overrides: Record<string, ThinkingCapabilityOverride>) => void
}

export function ThinkingCapabilitiesCard(props: ThinkingCapabilitiesCardProps) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState<string | null>(null)
  const models = tierModelsOfRoutes(props.routes)
  const title = t('Model thinking capabilities')
  const builtin = props.builtin ?? {}

  const restore = (model: string) => {
    const { [model]: _removed, ...rest } = props.overrides
    props.onChange(rest)
  }

  const sourceBadge = (source: 'builtin' | 'override' | 'unconfirmed') => {
    if (source === 'override') {
      return <Badge>{t('Overridden')}</Badge>
    }
    if (source === 'unconfirmed') {
      return <Badge variant='warning'>{t('Unconfirmed')}</Badge>
    }
    return <Badge variant='secondary'>{t('Built-in')}</Badge>
  }

  return (
    <section
      aria-label={title}
      className='bg-card min-w-0 overflow-hidden rounded-xl border'
    >
      <div className='flex flex-col gap-1.5 px-4 pt-5 pb-4 sm:px-5'>
        <div className='flex items-center gap-2'>
          <h3 className='text-base font-semibold'>{title}</h3>
          <Badge variant='secondary' className='tabular-nums'>
            {models.length}
          </Badge>
        </div>
        <p className='text-muted-foreground text-sm'>
          {t(
            "Smart routing uses this to decide which thinking efforts can be sent. Built-in values come from each vendor's official documentation and can be overridden per model."
          )}
        </p>
      </div>
      <Separator />
      {models.length === 0 ? (
        <EmptyState
          icon={Brain}
          title={t('No model is used by a virtual model yet')}
          className='min-h-40'
        />
      ) : null}
      {models.length > 0 && props.isPending ? (
        <LoadingState className='min-h-40' />
      ) : null}
      {models.length > 0 && props.isError ? (
        <ErrorState className='min-h-40' onRetry={props.onRetry} />
      ) : null}
      {models.length > 0 && !props.isPending && !props.isError ? (
        <StaticDataTable
          className='overflow-x-auto rounded-none border-0'
          tableProps={{ withContainer: false, 'aria-label': title }}
          tableClassName='min-w-[800px] table-fixed'
          headerRowClassName='bg-muted/35 hover:bg-muted/35'
          data={models}
          columns={[
            {
              id: 'model',
              header: t('Model'),
              className: 'w-64',
              cell: (model) => (
                <TruncatedCell tabIndex={0}>{model}</TruncatedCell>
              ),
            },
            {
              id: 'source',
              header: t('Source'),
              className: 'w-28',
              cell: (model) =>
                sourceBadge(
                  effectiveCapability(model, builtin, props.overrides).source
                ),
            },
            {
              id: 'efforts',
              header: t('Supported efforts'),
              cell: (model) => {
                const capability = effectiveCapability(
                  model,
                  builtin,
                  props.overrides
                )
                if (capability.noThinking) {
                  return (
                    <span className='text-muted-foreground'>
                      {t('No thinking')}
                    </span>
                  )
                }
                return (
                  <div className='flex flex-wrap items-center gap-1'>
                    {capability.efforts.map((effort) => (
                      <Badge
                        key={effort}
                        variant={
                          capability.source === 'unconfirmed'
                            ? 'outline'
                            : 'secondary'
                        }
                      >
                        {effort}
                      </Badge>
                    ))}
                  </div>
                )
              },
            },
            {
              id: 'actions',
              header: t('Actions'),
              className: 'w-48 text-right',
              cell: (model) => (
                <div
                  role='group'
                  aria-label={t('Actions for {{name}}', { name: model })}
                  className='flex items-center justify-end gap-0.5'
                >
                  {props.overrides[model] ? (
                    <>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => setEditing(model)}
                      >
                        {t('Modify')}
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => restore(model)}
                      >
                        {t('Restore built-in')}
                      </Button>
                    </>
                  ) : (
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => setEditing(model)}
                    >
                      {t('Override')}
                    </Button>
                  )}
                </div>
              ),
            },
          ]}
        />
      ) : null}
      {editing !== null ? (
        <CapabilityOverrideDialog
          model={editing}
          current={effectiveCapability(editing, builtin, props.overrides)}
          onOpenChange={(open) => !open && setEditing(null)}
          onSave={(override) => {
            props.onChange({ ...props.overrides, [editing]: override })
            setEditing(null)
          }}
        />
      ) : null}
    </section>
  )
}
