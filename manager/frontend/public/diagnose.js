// Frontend diagnostic script
console.log('=== Frontend Diagnostics Started ===')

// Check basic environment
console.log('1. Basic Environment:')
console.log('   - Vue Version:', typeof window.Vue !== 'undefined' ? 'Vue loaded' : 'Vue not loaded')
console.log('   - Current URL:', window.location.href)
console.log('   - User Agent:', navigator.userAgent)

// Check localStorage
console.log('2. Local Storage:')
console.log('   - Token:', localStorage.getItem('token'))
console.log('   - User:', localStorage.getItem('user'))

// Check network connection
console.log('3. Backend Connection:')
fetch('http://localhost:8080/api/profile')
  .then(response => {
    console.log('   - Backend Response Status:', response.status)
    if (response.status === 401) {
      console.log('   - Backend running normally (returned 401 unauthorized)')
    }
  })
  .catch(error => {
    console.log('   - Backend Connection Failed:', error.message)
  })

// Check routes
console.log('4. Available Test Routes:')
console.log('   - /test - Basic test page')
console.log('   - /simple-login - Simplified login page')
console.log('   - /login - Full login page')

console.log('=== Frontend Diagnostics Ended ===')
console.log('Please check the above information in browser console')

// Export to global for easy console access
window.diagnose = () => {
  console.clear()
  // Re-run diagnostics
  location.reload()
}