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
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'

import { getPolicyConfig } from './api'
import { getBuiltinThinkingCapabilities } from './smart-routing/api'
import { ThinkingCapabilitiesCard } from './smart-routing/capabilities-card'
import { tierModelsOfRoutes } from './smart-routing/capability'
import {
  parseSmartRoutes,
  parseThinkingCapabilities,
  serializeSmartRoutes,
  serializeThinkingCapabilities,
  SMART_ROUTES_OPTION_KEY,
  SMART_THINKING_CAPABILITIES_OPTION_KEY,
} from './smart-routing/lib'
import { SmartRoutesCard } from './smart-routing/routes-card'
import type {
  SmartRoute,
  ThinkingCapabilityOverride,
} from './smart-routing/types'
import { useSavePolicy } from './use-save-policy'

export function SmartRoutingSection() {
  const { t } = useTranslation()
  const mutation = useSavePolicy()
  const query = useQuery({
    queryKey: ['request-policy'],
    queryFn: getPolicyConfig,
    staleTime: Infinity,
  })
  const value = query.data?.options[SMART_ROUTES_OPTION_KEY] ?? ''
  const { routes } = parseSmartRoutes(value)
  const overrides = parseThinkingCapabilities(
    query.data?.options[SMART_THINKING_CAPABILITIES_OPTION_KEY] ?? ''
  )
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

  const saveRoutes = async (next: SmartRoute[]) => {
    await mutation.mutateAsync({
      [SMART_ROUTES_OPTION_KEY]: serializeSmartRoutes(next),
    })
  }

  const saveCapabilities = async (
    next: Record<string, ThinkingCapabilityOverride>
  ) => {
    await mutation.mutateAsync({
      [SMART_THINKING_CAPABILITIES_OPTION_KEY]:
        serializeThinkingCapabilities(next),
    })
  }

  return (
    <div className='min-w-0 space-y-4'>
      <SmartRoutesCard
        value={value}
        overrides={overrides}
        builtin={builtinQuery.data}
        onSave={saveRoutes}
        saving={mutation.isPending}
      />
      <ThinkingCapabilitiesCard
        routes={routes}
        builtin={builtinQuery.data}
        isPending={builtinQuery.isPending && tierModels.length > 0}
        isError={builtinQuery.isError}
        onRetry={() => void builtinQuery.refetch()}
        overrides={overrides}
        onSave={saveCapabilities}
        saving={mutation.isPending}
      />
    </div>
  )
}
