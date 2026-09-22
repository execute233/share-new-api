import { createFileRoute, redirect } from '@tanstack/react-router'
import { OpsMonitor } from '@/features/ops-monitor'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/ops-monitor/')({
  beforeLoad: () => {
    const user = useAuthStore.getState().auth.user
    if (!user || user.role < ROLE.ADMIN) throw redirect({ to: '/403' })
  },
  component: OpsMonitor,
})
