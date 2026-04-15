/**
 * Device detection utility
 * Used to determine current device type for responsive layout
 */

/**
 * Check if current device is mobile
 * @returns {boolean}
 */
export const isMobile = () => {
  // Detect via User-Agent
  const userAgent = navigator.userAgent || navigator.vendor || window.opera
  const mobileRegex = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i
  const isMobileUA = mobileRegex.test(userAgent)
  
  // Detect via screen width (fallback)
  const isMobileWidth = window.innerWidth < 768
  
  return isMobileUA || isMobileWidth
}

/**
 * Check if current device is tablet
 * @returns {boolean}
 */
export const isTablet = () => {
  const userAgent = navigator.userAgent || navigator.vendor || window.opera
  return /iPad|Android/i.test(userAgent) && window.innerWidth >= 768 && window.innerWidth < 1024
}

/**
 * Check if current device is desktop
 * @returns {boolean}
 */
export const isDesktop = () => {
  return !isMobile() && !isTablet()
}

/**
 * Check if current browser is WeChat
 * @returns {boolean}
 */
export const isWeChat = () => {
  const userAgent = navigator.userAgent || ''
  return /MicroMessenger/i.test(userAgent)
}

/**
 * Get device type
 * @returns {'mobile' | 'tablet' | 'desktop'}
 */
export const getDeviceType = () => {
  if (isMobile()) {
    return 'mobile'
  } else if (isTablet()) {
    return 'tablet'
  } else {
    return 'desktop'
  }
}

/**
 * Listen for window resize events
 * @param {Function} callback Callback function
 * @returns {Function} Function to remove listener
 */
export const onResize = (callback) => {
  let ticking = false
  
  const handler = () => {
    if (!ticking) {
      window.requestAnimationFrame(() => {
        callback(getDeviceType())
        ticking = false
      })
      ticking = true
    }
  }
  
  window.addEventListener('resize', handler)
  
  // Return function to remove listener
  return () => {
    window.removeEventListener('resize', handler)
  }
}
