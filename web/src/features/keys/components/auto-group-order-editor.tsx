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
import { ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { GripVertical, X } from 'lucide-react'
import { Reorder } from 'motion/react'
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
  type DragEvent,
} from 'react'
import { useTranslation } from 'react-i18next'

import { AutoGroupOrderItem } from '@/components/auto-group-order-item'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { cn } from '@/lib/utils'

import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { GroupRatioBadge } from './auto-group-visuals'

type AutoGroupOrderEditorProps = Omit<ComponentProps<'div'>, 'onChange'> & {
  value: string[]
  mode: 'inherit' | 'custom'
  options: ApiKeyGroupOption[]
  globalOptions: ApiKeyGroupOption[]
  maxCount: number
  hideInheritControls?: boolean
  variant?: 'compact'
  addLabel?: string
  onChange: (value: { groups: string[]; mode: 'inherit' | 'custom' }) => void
  'data-slot'?: string
  'data-form-root'?: string
}

export function AutoGroupOrderEditor(props: AutoGroupOrderEditorProps) {
  const { t } = useTranslation()
  const maxCount =
    Number.isInteger(props.maxCount) && props.maxCount > 0 ? props.maxCount : 5
  const isInheriting = props.mode === 'inherit'
  const atLimit = props.value.length >= maxCount
  const candidates = useMemo(
    () =>
      props.options.filter(
        (option) =>
          option.value !== 'auto' && !props.value.includes(option.value)
      ),
    [props.options, props.value]
  )

  const [draggedGroup, setDraggedGroup] = useState('')
  const [dropTarget, setDropTarget] = useState<{
    group: string
    position: 'before' | 'after'
  } | null>(null)

  const tagRefs = useRef(new Map<string, HTMLLIElement>())
  const pendingFocusGroup = useRef('')

  useEffect(() => {
    if (!pendingFocusGroup.current) return
    tagRefs.current.get(pendingFocusGroup.current)?.focus()
    pendingFocusGroup.current = ''
  }, [props.value])

  const handleAdd = (group: string) => {
    if (atLimit || props.value.includes(group)) return
    props.onChange({
      groups: [...props.value, group],
      mode: 'custom',
    })
  }

  const handleRemove = (group: string) => {
    props.onChange({
      groups: props.value.filter((item) => item !== group),
      mode: 'custom',
    })
  }

  const handleMove = (index: number, direction: 'up' | 'down') => {
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= props.value.length) return
    const next = [...props.value]
    ;[next[index], next[targetIndex]] = [next[targetIndex], next[index]]
    props.onChange({ groups: next, mode: 'custom' })
  }

  const resetDrag = () => {
    setDraggedGroup('')
    setDropTarget(null)
  }

  const getDropPosition = (event: DragEvent<HTMLElement>) => {
    const rect = event.currentTarget.getBoundingClientRect()
    return event.clientX < rect.left + rect.width / 2 ? 'before' : 'after'
  }

  const handleDrop = (event: DragEvent<HTMLElement>, targetGroup: string) => {
    event.preventDefault()
    const from = props.value.indexOf(draggedGroup)
    const target = props.value.indexOf(targetGroup)
    resetDrag()
    if (from < 0 || target < 0 || draggedGroup === targetGroup) return
    const insertAt = target + (getDropPosition(event) === 'after' ? 1 : 0)
    const next = props.value.filter((item) => item !== draggedGroup)
    next.splice(insertAt > from ? insertAt - 1 : insertAt, 0, draggedGroup)
    if (next.every((item, i) => item === props.value[i])) return
    props.onChange({ groups: next, mode: 'custom' })
  }

  if (props.variant === 'compact') {
    const addLabel = props.addLabel ?? t('Add group')
    return (
      <div
        id={props.id}
        data-slot={props['data-slot']}
        data-form-root={props['data-form-root']}
        role='group'
        tabIndex={-1}
        aria-label={props['aria-label'] || t('Auto group order')}
        aria-describedby={props['aria-describedby']}
        aria-invalid={props['aria-invalid']}
        className={cn(
          'aria-invalid:border-destructive aria-invalid:ring-destructive/20 flex flex-wrap items-center gap-1.5 rounded-lg border border-transparent p-1 aria-invalid:ring-3',
          props.className
        )}
      >
        <ol role='list' className='contents'>
          {props.value.map((group, index) => {
            const isDropTarget =
              dropTarget?.group === group && draggedGroup !== group
            return (
              <li
                key={group}
                ref={(node) => {
                  if (node) tagRefs.current.set(group, node)
                  else tagRefs.current.delete(group)
                }}
                tabIndex={0}
                aria-label={t('{{index}}. {{group}}', {
                  index: index + 1,
                  group,
                })}
                aria-keyshortcuts='Alt+ArrowLeft Alt+ArrowRight'
                draggable={props.value.length > 1}
                onDragStart={(event) => {
                  event.dataTransfer?.setData('text/plain', group)
                  setDraggedGroup(group)
                }}
                onDragOver={(event) => {
                  if (!draggedGroup) return
                  event.preventDefault()
                  setDropTarget({ group, position: getDropPosition(event) })
                }}
                onDrop={(event) => handleDrop(event, group)}
                onDragEnd={resetDrag}
                onKeyDown={(event) => {
                  if (!event.altKey) return
                  if (event.key === 'ArrowLeft') {
                    event.preventDefault()
                    if (index === 0) return
                    pendingFocusGroup.current = group
                    handleMove(index, 'up')
                  } else if (event.key === 'ArrowRight') {
                    event.preventDefault()
                    if (index === props.value.length - 1) return
                    pendingFocusGroup.current = group
                    handleMove(index, 'down')
                  }
                }}
                className={cn(
                  'bg-muted/30 flex min-w-0 items-center gap-1.5 rounded-md border px-2 py-1',
                  props.value.length > 1 &&
                    'cursor-grab active:cursor-grabbing',
                  draggedGroup === group && 'opacity-50',
                  isDropTarget &&
                    dropTarget.position === 'before' &&
                    'border-l-primary border-l-2',
                  isDropTarget &&
                    dropTarget.position === 'after' &&
                    'border-r-primary border-r-2'
                )}
              >
                <GripVertical
                  aria-hidden='true'
                  className='text-muted-foreground size-3.5 shrink-0'
                />
                <span
                  aria-hidden='true'
                  className='bg-primary/10 text-primary flex size-4 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold tabular-nums'
                >
                  {index + 1}
                </span>
                <span className='max-w-40 truncate text-xs font-medium'>
                  {group}
                </span>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-xs'
                  aria-label={t('Remove {{group}}', { group })}
                  onClick={() => handleRemove(group)}
                >
                  <X aria-hidden='true' />
                </Button>
              </li>
            )
          })}
        </ol>
        <ApiKeyGroupCombobox
          compact
          options={candidates}
          value={undefined}
          onValueChange={handleAdd}
          placeholder={props.value.length === 0 ? addLabel : undefined}
          disabled={atLimit || candidates.length === 0}
          aria-label={addLabel}
        />
      </div>
    )
  }

  return (
    <div
      id={props.id}
      data-slot={props['data-slot']}
      data-form-root={props['data-form-root']}
      role='group'
      tabIndex={-1}
      aria-label={props['aria-label'] || t('Auto group order')}
      aria-describedby={props['aria-describedby']}
      aria-invalid={props['aria-invalid']}
      className={cn('flex flex-col gap-3', props.className)}
    >
      {!props.hideInheritControls && (
        <div className='flex items-center justify-between gap-3'>
          <p className='text-muted-foreground text-xs' aria-live='polite'>
            {isInheriting
              ? t('Using the complete global Auto order ({{count}} groups)', {
                  count: props.globalOptions.length,
                })
              : t('{{count}} / {{max}} groups selected', {
                  count: props.value.length,
                  max: maxCount,
                })}
          </p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={isInheriting}
            onClick={() => {
              props.onChange({ groups: [], mode: 'inherit' })
            }}
          >
            {t('Restore global Auto')}
          </Button>
        </div>
      )}

      <ApiKeyGroupCombobox
        options={candidates}
        value={undefined}
        onValueChange={handleAdd}
        placeholder={
          atLimit
            ? t('Maximum {{max}} groups selected', { max: maxCount })
            : (props.addLabel ?? t('Add Auto group'))
        }
        disabled={atLimit || candidates.length === 0}
        aria-label={props.addLabel}
      />

      {isInheriting && props.globalOptions.length === 0 && (
        <Empty className='min-h-28 border'>
          <EmptyHeader>
            <EmptyTitle>{t('Inherit global Auto order')}</EmptyTitle>
            <EmptyDescription>
              {t('No available groups in the global Auto order.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}

      {isInheriting && props.globalOptions.length > 0 && (
        <ol
          data-slot='global-auto-order'
          aria-label={t('Inherit global Auto order')}
          className='flex max-h-24 flex-wrap content-start gap-1.5 overflow-y-auto'
        >
          {props.globalOptions.map((option, index) => (
            <li key={option.value} className='flex min-w-0 items-center gap-1'>
              {index > 0 && (
                <HugeiconsIcon
                  icon={ArrowRight01Icon}
                  strokeWidth={2}
                  aria-hidden='true'
                  data-slot='global-auto-order-connector'
                  className='text-muted-foreground size-3.5 shrink-0'
                />
              )}
              <span
                data-slot='global-auto-order-chip'
                title={option.desc}
                className='bg-muted/30 flex min-w-0 items-center gap-1.5 rounded-md border px-2 py-1'
              >
                <span
                  data-slot='global-auto-order-index'
                  aria-hidden='true'
                  className='bg-primary/10 text-primary flex size-4 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold tabular-nums'
                >
                  {index + 1}
                </span>
                <span
                  data-slot='global-auto-order-name'
                  className='max-w-40 truncate text-xs font-medium'
                >
                  {option.label}
                </span>
                {option.desc && (
                  <span
                    data-slot='global-auto-order-description'
                    className='sr-only'
                  >
                    {option.desc}
                  </span>
                )}
                <GroupRatioBadge ratio={option.ratio} />
              </span>
            </li>
          ))}
        </ol>
      )}

      {!isInheriting &&
        !props.hideInheritControls &&
        props.value.length === 0 && (
          <Empty className='min-h-24 border'>
            <EmptyHeader>
              <EmptyTitle>{t('Auto group order')}</EmptyTitle>
              <EmptyDescription>
                {t(
                  'No valid custom Auto groups remain. Add a group or restore global Auto.'
                )}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}

      {!isInheriting && props.value.length > 0 && (
        <Reorder.Group
          axis='y'
          values={props.value}
          onReorder={(groups) => props.onChange({ groups, mode: 'custom' })}
          className='flex flex-col gap-2'
        >
          {props.value.map((group, index) => (
            <AutoGroupOrderItem
              key={group}
              group={group}
              index={index}
              count={props.value.length}
              onMove={handleMove}
              onRemove={handleRemove}
            />
          ))}
        </Reorder.Group>
      )}
    </div>
  )
}
