import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { useMemo } from 'react'

export function useRouter() {
  const navigate = useNavigate()
  const location = useLocation()
  const params = useParams()

  const router = useMemo(() => {
    const query = Object.fromEntries(new URLSearchParams(location.search))

    return {
      // Next.js specific properties
      pathname: location.pathname,
      query: { ...params, ...query },
      asPath: location.pathname + location.search,
      isReady: true,
      route: location.pathname,

      // Navigation methods
      push: (path: string) => navigate(path),
      replace: (path: string) => navigate(path, { replace: true }),
      reload: () => window.location.reload(),
      back: () => navigate(-1),
      forward: () => navigate(1),
      
      // Stubbed events (Next.js has these, optional for Vite but good for compatibility)
      events: {
        on: () => {},
        off: () => {},
        emit: () => {},
      },
    }
  }, [navigate, location, params])

  return router
}
