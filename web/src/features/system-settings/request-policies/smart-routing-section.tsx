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
import { getBuiltinThinkingCapabilities } from './smart-routing/api'
import { ThinkingCapabilitiesCard } from './smart-routing/capabilities-card'
import { tierModelsOfRoutes } from './smart-routing/capability'
import {
  parseSmartRoutes,
  parseThinkingCapabilities,
  serializeSmartRoutes,
  serializeThinkingCapabilities,
  smartRoutesErrorMessage,
  SMART_ROUTES_OPTION_KEY,
  SMART_THINKING_CAPABILITIES_OPTION_KEY,
} from './smart-routing/lib'
import { SmartRoutesCard } from './smart-routing/routes-card'
import { useSavePolicy } from './use-save-policy'

export function SmartRoutingSection() {
  const { t } = useTranslation()
  const mutation = useSavePolicy()
  const [draft, setDraft] = useState<string | null>(null)
  const [capabilitiesDraft, setCapabilitiesDraft] = useState<string | null>(
    null
  )
  const query = useQuery({
    queryKey: ['request-policy'],
    queryFn: getPolicyConfig,
    staleTime: Infinity,
  })
  const baseline = query.data?.options[SMART_ROUTES_OPTION_KEY] ?? ''
  const capabilitiesBaseline =
    query.data?.options[SMART_THINKING_CAPABILITIES_OPTION_KEY] ?? ''
  const value = draft ?? baseline
  const capabilitiesValue = capabilitiesDraft ?? capabilitiesBaseline
  const { routes } = parseSmartRoutes(value)
  const overrides = parseThinkingCapabilities(capabilitiesValue)
  const tierModels = tierModelsOfRoutes(routes).sort()
  const builtinQuery = useQuery({
    queryKey: ['smart-routing', 'builtin-capabilities', tierModels],
    queryFn: () => getBuiltinThinkingCapabilities(tierModels),
    enabled: tierModels.length > 0,
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

  const save = async () => {
    const next = parseSmartRoutes(value)
    if (next.error) {
      toast.error(smartRoutesErrorMessage(t, next.error))
      return
    }
    const serialized = serializeSmartRoutes(next.routes)
    const serializedCapabilities = serializeThinkingCapabilities(overrides)
    const routesChanged =
      serialized !== serializeSmartRoutes(parseSmartRoutes(baseline).routes)
    const capabilitiesChanged =
      serializedCapabilities !==
      serializeThinkingCapabilities(
        parseThinkingCapabilities(capabilitiesBaseline)
      )
    if (!routesChanged && !capabilitiesChanged) {
      toast.info(t('No changes to save'))
      return
    }
    try {
      await mutation.mutateAsync({
        ...(routesChanged && { [SMART_ROUTES_OPTION_KEY]: serialized }),
        ...(capabilitiesChanged && {
          [SMART_THINKING_CAPABILITIES_OPTION_KEY]: serializedCapabilities,
        }),
      })
      setDraft(null)
      setCapabilitiesDraft(null)
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
        overrides={overrides}
        builtin={builtinQuery.data}
        onChange={setDraft}
      />
      <ThinkingCapabilitiesCard
        routes={routes}
        builtin={builtinQuery.data}
        isPending={builtinQuery.isPending && tierModels.length > 0}
        isError={builtinQuery.isError}
        onRetry={() => void builtinQuery.refetch()}
        overrides={overrides}
        onChange={(next) =>
          setCapabilitiesDraft(serializeThinkingCapabilities(next))
        }
      />
    </div>
  )
}
