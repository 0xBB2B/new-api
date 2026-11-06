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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { handleServerError } from '@/lib/handle-server-error'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { getPolicyConfig } from './api'
import {
  parseSmartRoutes,
  serializeSmartRoutes,
  smartRoutesErrorMessage,
  SMART_ROUTES_OPTION_KEY,
} from './smart-routing/lib'
import { SmartRoutesCard } from './smart-routing/routes-card'
import type { ThinkingCapabilityOverride } from './smart-routing/types'
import { useSavePolicy } from './use-save-policy'

const NO_OVERRIDES: Record<string, ThinkingCapabilityOverride> = {}

export function SmartRoutingSection() {
  const { t } = useTranslation()
  const mutation = useSavePolicy()
  const [draft, setDraft] = useState<string | null>(null)
  const query = useQuery({
    queryKey: ['request-policy'],
    queryFn: getPolicyConfig,
    staleTime: Infinity,
  })
  if (query.isPending) return <LoadingState />
  if (query.isError) {
    return (
      <ErrorState
        title={t('Failed to load settings')}
        onRetry={() => void query.refetch()}
      />
    )
  }

  const baseline = query.data.options[SMART_ROUTES_OPTION_KEY] ?? ''
  const value = draft ?? baseline

  const save = async () => {
    const next = parseSmartRoutes(value)
    if (next.error) {
      toast.error(smartRoutesErrorMessage(t, next.error))
      return
    }
    const serialized = serializeSmartRoutes(next.routes)
    if (
      serialized === serializeSmartRoutes(parseSmartRoutes(baseline).routes)
    ) {
      toast.info(t('No changes to save'))
      return
    }
    try {
      await mutation.mutateAsync({ [SMART_ROUTES_OPTION_KEY]: serialized })
      setDraft(null)
    } catch (error) {
      handleServerError(error)
    }
  }

  return (
    <div className='min-w-0 space-y-4'>
      <SettingsPageFormActions
        onSave={() => void save()}
        isSaving={mutation.isPending}
      />
      <SmartRoutesCard
        value={value}
        overrides={NO_OVERRIDES}
        onChange={(next) => {
          setDraft(next)
        }}
      />
    </div>
  )
}
