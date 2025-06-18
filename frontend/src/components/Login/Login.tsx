import React, { useState } from 'react';
import { login, LoginCredentials } from '../../api/axiosConfig';
import { useAuth } from '../../contexts/AuthContext';

const Login: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const { setIsUserLoggedIn } = useAuth();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(''); // Clear previous errors
    
    try {
      console.log('Attempting login...');
      const credentials: LoginCredentials = { username, password };
      const response = await login(credentials);

      console.log('Login successful, redirecting...');
      localStorage.setItem('token', response.token);
      setIsUserLoggedIn(true);
      // Force page reload to reinitialize auth state
      window.location.href = '/';
    } catch (error: any) {
      console.error('Login error:', error);
      if (error.response) {
        // Server responded with error status
        const status = error.response.status;
        const message = error.response.data?.error || error.response.statusText;
        setError(`Login failed: ${status} ${message}`);
      } else if (error.request) {
        // Request was made but no response received
        console.error('Network error during login:', error);
        setError('Network error - cannot connect to server');
      } else {
        // Something else happened
        setError('Login failed - unexpected error');
      }
    }
  };

  return (
    <div className="container d-flex align-items-center justify-content-center min-vh-100">
      <div className="card bg-black border-success text-secondary p-4 shadow-sm" style={{ maxWidth: '400px', width: '100%' }}>
        <h2 className="text-center mb-4">Login</h2>
        {error && <p className="text-danger text-center">{error}</p>}
        <form onSubmit={handleSubmit}>
          <div className="mb-3">
            <label htmlFor="username" className="form-label">Username:</label>
            <input
              type="text"
              className="form-control bg-black border-dark text-success"
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>
          <div className="mb-3">
            <label htmlFor="password" className="form-label">Password:</label>
            <input
              type="password"
              className="form-control bg-black border-dark text-success"
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <button type="submit" className="btn btn-success w-100">Login</button>
        </form>
      </div>
    </div>
  );
};

export default Login;
