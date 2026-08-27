import { createFileRoute, redirect } from '@tanstack/react-router'

import { Proxies } from '@/features/proxies'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/proxies/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) throw redirect({ to: '/403' })
  },
  component: Proxies,
})
