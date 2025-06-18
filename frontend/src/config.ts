// Backend configuration
const BACKEND_PORT = '3008';

/**
 * Returns the appropriate backend URL based on environment and how the frontend is being accessed.
 * In production (Docker), uses relative paths to go through nginx proxy.
 * In development, uses direct localhost connection.
 */
export const getBackendUrl = (): string => {
  // Check if we're in production (Docker environment)
  const isProduction = process.env.NODE_ENV === 'production';
  
  console.log('Environment:', process.env.NODE_ENV);
  
  // In production (Docker), use relative paths to go through nginx proxy
  if (isProduction) {
    console.log('Using relative paths for nginx proxy');
    return '';
  }

  // In development, use direct connection to backend
  const protocol = window.location.protocol;
  const hostname = window.location.hostname;

  console.log('Frontend hostname:', hostname);
  console.log('Frontend protocol:', protocol);

  // If we're accessing from localhost, use localhost
  if (hostname === 'localhost' || hostname === '127.0.0.1') {
    const url = `http://localhost:${BACKEND_PORT}`;
    console.log('Using backend URL:', url);
    return url;
  }

  // Otherwise, use the current hostname
  const url = `${protocol}//${hostname}:${BACKEND_PORT}`;
  console.log('Using backend URL:', url);
  return url;
};

export const API_BASE_URL = getBackendUrl();

