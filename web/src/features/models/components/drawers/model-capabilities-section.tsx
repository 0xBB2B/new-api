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
import { useEffect, useRef, useState } from 'react'
import { useController } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { SideDrawerSection } from '@/components/drawer-layout'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import {
  MODEL_INPUT_MODALITIES,
  MODEL_OUTPUT_MODALITIES,
  MODEL_REASONING_EFFORTS,
  capabilityToFormValues,
  formValuesToCapability,
  parseModelCapabilityJson,
  serializeModelCapability,
  type ModelCapabilityError,
  type ModelCapabilityFormValues,
} from '../../lib/model-capabilities'
import type { ModelFormValues } from '../../lib/model-form'

const EMPTY_VALUES = capabilityToFormValues({})

const MODALITY_LABELS: Record<string, string> = {
  text: 'Text',
  image: 'Image',
  file: 'File',
  audio: 'Audio',
}

function valuesFromText(text: string) {
  if (text.trim() === '') return EMPTY_VALUES
  const result = parseModelCapabilityJson(text)
  return result.ok ? capabilityToFormValues(result.value) : null
}

function CheckboxGroup(props: {
  options: readonly string[]
  value: string[]
  disabled?: boolean
  translate?: boolean
  onChange: (value: string[]) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-wrap gap-x-4 gap-y-2'>
      {props.options.map((option) => (
        <Label key={option} className='flex items-center gap-2 font-normal'>
          <Checkbox
            nativeButton
            render={<button type='button' />}
            checked={props.value.includes(option)}
            disabled={props.disabled}
            onCheckedChange={(checked) =>
              props.onChange(
                checked === true
                  ? [...props.value, option]
                  : props.value.filter((v) => v !== option)
              )
            }
          />
          {props.translate ? t(MODALITY_LABELS[option]) : option}
        </Label>
      ))}
    </div>
  )
}

export function ModelCapabilitiesSection() {
  const { t } = useTranslation()
  const { field } = useController<ModelFormValues, 'capabilities'>({
    name: 'capabilities',
  })
  const [mode, setMode] = useState<'form' | 'json'>('form')
  const [values, setValues] = useState<ModelCapabilityFormValues>(
    () => valuesFromText(field.value) ?? EMPTY_VALUES
  )
  const written = useRef(field.value)

  useEffect(() => {
    if (field.value === written.current) return
    written.current = field.value
    const next = valuesFromText(field.value)
    if (next) setValues(next)
  }, [field.value])

  const write = (text: string) => {
    written.current = text
    field.onChange(text)
  }
  const update = (patch: Partial<ModelCapabilityFormValues>) => {
    const next = { ...values, ...patch }
    setValues(next)
    write(serializeModelCapability(formValuesToCapability(next)))
  }

  const parsed =
    field.value.trim() === '' ? null : parseModelCapabilityJson(field.value)
  const errors: ModelCapabilityError[] =
    parsed?.ok === false ? parsed.errors : []
  const isSyntaxError = errors.some((e) => e.key === 'Invalid JSON')
  const errorsAt = (path: string) => errors.filter((e) => e.path === path)
  const renderErrors = (list: ModelCapabilityError[]) =>
    list.map((e) => (
      <p key={`${e.path}:${e.key}`} className='text-destructive text-sm'>
        {t(e.key, e.params)}
      </p>
    ))
  const renderNumberErrors = (path: string) =>
    errorsAt(path).length > 0 && (
      <p className='text-destructive text-sm'>
        {t('Enter an integer greater than 0')}
      </p>
    )

  const switchMode = () => {
    if (mode === 'form') {
      setMode('json')
      return
    }
    const next = valuesFromText(field.value)
    if (!next) return
    setValues(next)
    setMode('form')
  }

  return (
    <SideDrawerSection>
      <div className='flex items-center justify-between gap-2'>
        <h3 className='text-sm font-semibold'>{t('Model capabilities')}</h3>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={mode === 'json' && errors.length > 0}
          onClick={switchMode}
        >
          {mode === 'form' ? t('JSON mode') : t('Visual mode')}
        </Button>
      </div>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Values set here are returned to callers through /v1/models. Unset items are not returned.'
        )}
      </p>

      {mode === 'json' ? (
        <div className='flex flex-col gap-2'>
          <Label>{t('Capability configuration')}</Label>
          <JsonCodeEditor
            value={field.value.trim() === '' ? '{}' : field.value}
            onChange={write}
            ariaLabel={t('Capability configuration')}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Same shape as /v1/models returns. Missing fields are unset; write "supported_efforts": [] for no thinking support.'
            )}
          </p>
          {isSyntaxError ? (
            <p className='text-destructive text-sm'>
              {t(
                'JSON format error. Fix it before saving or switching back to visual mode.'
              )}
            </p>
          ) : (
            renderErrors(errors)
          )}
        </div>
      ) : (
        <>
          <div className='flex flex-col gap-2'>
            <Label>{t('Input modalities')}</Label>
            <CheckboxGroup
              options={MODEL_INPUT_MODALITIES}
              value={values.input_modalities}
              translate
              onChange={(value) => update({ input_modalities: value })}
            />
            <p className='text-muted-foreground text-sm'>
              {t('Leave all unchecked to keep this unset.')}
            </p>
            {renderErrors(errorsAt('architecture.input_modalities'))}
          </div>

          <div className='flex flex-col gap-2'>
            <Label>{t('Output modalities')}</Label>
            <CheckboxGroup
              options={MODEL_OUTPUT_MODALITIES}
              value={values.output_modalities}
              translate
              onChange={(value) => update({ output_modalities: value })}
            />
            <p className='text-muted-foreground text-sm'>
              {t('Leave all unchecked to keep this unset.')}
            </p>
            {renderErrors(errorsAt('architecture.output_modalities'))}
          </div>

          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='flex flex-col gap-2'>
              <Label htmlFor='capabilities-context-length'>
                {t('Context length')}
              </Label>
              <Input
                id='capabilities-context-length'
                type='number'
                placeholder={t('e.g. 1050000')}
                value={values.context_length}
                aria-invalid={errorsAt('context_length').length > 0}
                onChange={(e) => update({ context_length: e.target.value })}
              />
              <p className='text-muted-foreground text-sm'>
                {t('In tokens. Leave empty to keep this unset.')}
              </p>
              {renderNumberErrors('context_length')}
            </div>
            <div className='flex flex-col gap-2'>
              <Label htmlFor='capabilities-max-output'>
                {t('Max output tokens')}
              </Label>
              <Input
                id='capabilities-max-output'
                type='number'
                placeholder={t('e.g. 128000')}
                value={values.max_completion_tokens}
                aria-invalid={
                  errorsAt('top_provider.max_completion_tokens').length > 0
                }
                onChange={(e) =>
                  update({ max_completion_tokens: e.target.value })
                }
              />
              <p className='text-muted-foreground text-sm'>
                {t('In tokens. Leave empty to keep this unset.')}
              </p>
              {renderNumberErrors('top_provider.max_completion_tokens')}
            </div>
          </div>

          <div className='flex flex-col gap-2'>
            <Label>{t('Thinking levels')}</Label>
            <Label className='font-normal'>
              <Switch
                checked={values.no_reasoning}
                onCheckedChange={(checked) => update({ no_reasoning: checked })}
              />
              {t('No thinking support')}
            </Label>
            <CheckboxGroup
              options={MODEL_REASONING_EFFORTS}
              value={values.supported_efforts}
              disabled={values.no_reasoning}
              onChange={(value) => update({ supported_efforts: value })}
            />
            <p className='text-muted-foreground text-sm'>
              {t(
                'Check the levels this model supports. With the switch off and no level checked, this stays unset.'
              )}
            </p>
            {renderErrors(errorsAt('reasoning.supported_efforts'))}
          </div>
        </>
      )}
    </SideDrawerSection>
  )
}
