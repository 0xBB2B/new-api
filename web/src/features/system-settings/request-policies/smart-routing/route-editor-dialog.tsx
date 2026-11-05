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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown } from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import { useForm, useWatch, type FieldPath } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { ComboboxInput } from '@/components/ui/combobox-input'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { getEnabledModels } from '@/features/channels/api'
import { AutoGroupOrderEditor } from '@/features/keys/components/auto-group-order-editor'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { getGroups } from '@/features/users/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import { validateSmartRoute } from './api'
import { routeEndpoints, SMART_TIER_IDS } from './lib'
import {
  createRouteFormSchema,
  emptyRouteFormValues,
  formValuesToRoute,
  routeToFormValues,
  type RouteFormValues,
} from './route-form'
import type { SmartRoute, SmartTierId } from './types'

const ROUTE_FORM_ID = 'smart-route-form'
const ADVANCED_FIELDS = [
  'downgrade_max_history_tokens',
  'confidence_threshold',
  'simple_max_message_chars',
]
const KEEP_EFFORT = 'keep'

type ModelFieldName =
  | `tiers.${SmartTierId}.model`
  | 'scorer.model'
  | 'advisor.model'
type GroupsFieldName =
  | `tiers.${SmartTierId}.groups`
  | 'scorer.groups'
  | 'advisor.groups'
type NumberFieldName = Extract<
  FieldPath<RouteFormValues>,
  | 'scorer.timeout_ms'
  | 'advisor.after_dissatisfied'
  | 'downgrade_max_history_tokens'
  | 'confidence_threshold'
  | 'simple_max_message_chars'
>

type RouteEditorDialogProps = {
  route: SmartRoute | null
  otherNames: string[]
  onOpenChange: (open: boolean) => void
  onSave: (route: SmartRoute) => void
}

export function RouteEditorDialog(props: RouteEditorDialogProps) {
  const { t } = useTranslation()
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const { models: pricingModels } = usePricingData()
  const modelsQuery = useQuery({
    queryKey: ['smart-routing', 'enabled-models'],
    queryFn: async () => requireServerSuccess(await getEnabledModels()),
  })
  const groupsQuery = useQuery({
    queryKey: ['smart-routing', 'groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
  })
  const modelOptions = useMemo(
    () => (modelsQuery.data?.data ?? []).map((m) => ({ value: m, label: m })),
    [modelsQuery.data]
  )
  const groupOptions = useMemo(
    () => (groupsQuery.data?.data ?? []).map((g) => ({ value: g, label: g })),
    [groupsQuery.data]
  )
  const schema = useMemo(() => createRouteFormSchema(t), [t])
  const form = useForm<RouteFormValues>({
    resolver: zodResolver(schema),
    defaultValues: props.route
      ? routeToFormValues(props.route)
      : emptyRouteFormValues(),
  })
  const values = useWatch({ control: form.control })
  const endpoints = routeEndpoints(
    formValuesToRoute(values as RouteFormValues),
    pricingModels
  )

  const closedRef = useRef(false)
  const close = (open: boolean) => {
    if (!open) closedRef.current = true
    props.onOpenChange(open)
  }

  const submit = async (formValues: RouteFormValues) => {
    const route = formValuesToRoute(formValues)
    if (props.otherNames.includes(route.name)) {
      form.setError('name', {
        type: 'server',
        message: t('A virtual model named {{name}} already exists', {
          name: route.name,
        }),
      })
      return
    }
    try {
      const result = await validateSmartRoute({
        route,
      })
      if (closedRef.current) return
      if (result.errors.length > 0) {
        for (const error of result.errors) {
          form.setError(error.field as FieldPath<RouteFormValues>, {
            type: 'server',
            message: t(error.message.key, error.message.params),
          })
        }
        if (result.errors.some((e) => ADVANCED_FIELDS.includes(e.field))) {
          setAdvancedOpen(true)
        }
        return
      }
      props.onSave(route)
      close(false)
    } catch (error) {
      handleServerError(error)
    }
  }

  const openAdvancedOnError = (errors: object) => {
    if (ADVANCED_FIELDS.some((field) => field in errors)) setAdvancedOpen(true)
  }

  const modelField = (
    name: ModelFieldName,
    label: string,
    ariaLabel: string = label
  ) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <ComboboxInput
              options={modelOptions}
              value={field.value}
              onValueChange={field.onChange}
              placeholder={t('Select a model')}
              openOnFocus={false}
              aria-label={ariaLabel}
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )

  const groupsField = (name: GroupsFieldName) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{t('Group order')}</FormLabel>
          <FormControl>
            <AutoGroupOrderEditor
              value={field.value}
              mode='custom'
              options={groupOptions}
              globalOptions={[]}
              maxCount={groupOptions.length}
              hideInheritControls
              addLabel={t('Add group')}
              aria-label={t('Group order')}
              onChange={({ groups }) => field.onChange(groups)}
            />
          </FormControl>
          <FormDescription>
            {t(
              'Groups are tried in order. The next group is used when every channel in the current one fails.'
            )}
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )

  const numberField = (
    name: NumberFieldName,
    label: string,
    hint?: string,
    step?: string
  ) => (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input
              type='number'
              step={step}
              name={field.name}
              ref={field.ref}
              onBlur={field.onBlur}
              value={Number.isNaN(field.value) ? '' : field.value}
              onChange={(e) =>
                field.onChange(
                  e.target.value === '' ? Number.NaN : Number(e.target.value)
                )
              }
            />
          </FormControl>
          {hint ? <FormDescription>{hint}</FormDescription> : null}
          <FormMessage />
        </FormItem>
      )}
    />
  )

  const tierTitles: Record<SmartTierId, string> = {
    simple: t('Simple tier'),
    normal: t('Normal tier'),
    hard: t('Hard tier'),
  }
  const effortItems = [
    { value: KEEP_EFFORT, label: t('Keep client value') },
    { value: 'low', label: 'low' },
    { value: 'medium', label: 'medium' },
    { value: 'high', label: 'high' },
  ]
  const tierItems = SMART_TIER_IDS.map((id) => ({
    value: id,
    label: tierTitles[id],
  }))

  return (
    <Dialog
      open
      onOpenChange={close}
      title={props.route ? t('Edit virtual model') : t('Add virtual model')}
      contentClassName='sm:max-w-5xl'
      footer={
        <>
          <Button type='button' variant='outline' onClick={() => close(false)}>
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form={ROUTE_FORM_ID}
            disabled={form.formState.isSubmitting}
          >
            {t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={ROUTE_FORM_ID}
          noValidate
          onSubmit={form.handleSubmit(submit, openAdvancedOnError)}
          className='flex min-w-0 flex-col gap-6'
        >
          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel required>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('e.g. auto-gpt')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormItem>
              <FormLabel htmlFor='smart-route-endpoint'>
                {t('Interface')}
              </FormLabel>
              <Input
                id='smart-route-endpoint'
                readOnly
                value={endpoints.join(' / ')}
                placeholder={t('Detected after you pick models')}
              />
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Detected from the selected models. All models of a virtual model must use the same endpoint'
                )}
              </p>
            </FormItem>
          </div>

          <Separator />

          <section className='flex flex-col gap-4'>
            <div>
              <h4 className='text-sm font-semibold'>{t('Tiers')}</h4>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Each new user message is scored and goes to one tier. The default tier is used when scoring fails, times out or is unsure.'
                )}
              </p>
            </div>
            {SMART_TIER_IDS.map((id) => (
              <div key={id} className='flex flex-col gap-4'>
                <div className='grid gap-4 sm:grid-cols-2'>
                  {modelField(
                    `tiers.${id}.model`,
                    t('{{tier}} model', { tier: tierTitles[id] })
                  )}
                  <FormField
                    control={form.control}
                    name={`tiers.${id}.effort`}
                    render={({ field, fieldState }) => (
                      <FormItem>
                        <FormLabel>{t('Reasoning effort')}</FormLabel>
                        <Select
                          items={effortItems}
                          value={field.value || KEEP_EFFORT}
                          onValueChange={(value) =>
                            field.onChange(value === KEEP_EFFORT ? '' : value)
                          }
                        >
                          <SelectTrigger
                            className='w-full'
                            aria-label={t('Reasoning effort')}
                            aria-invalid={!!fieldState.error}
                          >
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {effortItems.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {item.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        {id === 'simple' && (
                          <FormDescription>
                            {t(
                              'Keep client value keeps the effort sent by the client'
                            )}
                          </FormDescription>
                        )}
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
                {groupsField(`tiers.${id}.groups`)}
              </div>
            ))}
            <FormField
              control={form.control}
              name='default_tier'
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel>{t('Default tier')}</FormLabel>
                  <Select
                    items={tierItems}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <SelectTrigger
                      className='w-full sm:w-60'
                      aria-label={t('Default tier')}
                      aria-invalid={!!fieldState.error}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {tierItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          </section>

          <Separator />

          <section className='flex flex-col gap-4'>
            <div>
              <h4 className='text-sm font-semibold'>{t('Scorer model')}</h4>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Called once for each new user message. It is billed at the scorer model price as a separate usage record.'
                )}
              </p>
            </div>
            <div className='grid gap-4 sm:grid-cols-2'>
              {modelField('scorer.model', t('Model'), t('Scorer model'))}
              {numberField(
                'scorer.timeout_ms',
                t('Timeout (ms)'),
                t('The default tier is used on timeout')
              )}
            </div>
            {groupsField('scorer.groups')}
          </section>

          <Separator />

          <section className='flex flex-col gap-4'>
            <div>
              <h4 className='text-sm font-semibold'>{t('Advisor')}</h4>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'When a user is dissatisfied several messages in a row in one conversation, the advisor answers this turn.'
                )}
              </p>
            </div>
            <div className='grid gap-4 sm:grid-cols-2'>
              {modelField('advisor.model', t('Model'), t('Advisor'))}
              {numberField(
                'advisor.after_dissatisfied',
                t('Dissatisfied messages before the advisor answers')
              )}
            </div>
            {groupsField('advisor.groups')}
          </section>

          <Separator />

          <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
            <CollapsibleTrigger
              render={<Button type='button' variant='ghost' size='sm' />}
            >
              <ChevronDown
                aria-hidden='true'
                data-icon='inline-start'
                className={advancedOpen ? '' : '-rotate-90'}
              />
              {t('Advanced settings')}
            </CollapsibleTrigger>
            <CollapsibleContent keepMounted className='pt-4'>
              <div className='grid gap-4 sm:grid-cols-3'>
                {numberField(
                  'downgrade_max_history_tokens',
                  t('Downgrade history token limit'),
                  t(
                    'No downgrade once the history exceeds this length, to avoid re-reading the whole history at full price'
                  )
                )}
                {numberField(
                  'confidence_threshold',
                  t('Confidence threshold'),
                  t(
                    'The default tier is used when the scorer is less sure than this'
                  ),
                  '0.05'
                )}
                {numberField(
                  'simple_max_message_chars',
                  t('Simple message length limit (characters)'),
                  t(
                    'A latest message longer than this is not treated as simple'
                  )
                )}
              </div>
            </CollapsibleContent>
          </Collapsible>
        </form>
      </Form>
    </Dialog>
  )
}
