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
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  describeUserTooltip,
  resolveUserName,
  type UserIdentity,
} from '@/lib/user-identity'
import { cn } from '@/lib/utils'

interface UserIdentityLabelProps {
  user: UserIdentity
  masked?: boolean
  className?: string
}

function UserIdentityLabel({
  user,
  masked,
  className,
}: UserIdentityLabelProps) {
  const { t } = useTranslation()

  if (masked) {
    return <span className={className}>••••</span>
  }

  const name = resolveUserName(user, t)

  return (
    <Tooltip>
      <TooltipTrigger render={<span className={cn('truncate', className)} />}>
        {name}
      </TooltipTrigger>
      <TooltipContent>
        {describeUserTooltip(user, t).map((line) => (
          <div key={line}>{line}</div>
        ))}
      </TooltipContent>
    </Tooltip>
  )
}

export { UserIdentityLabel }
