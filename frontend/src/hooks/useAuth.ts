// src/hooks/useAuth.ts
import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { checkAuth } from '../api/axiosConfig';

const useAuth = () => {
  const [isAuthenticated, setIsAuthenticated] = useState<boolean | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    const performAuthCheck = async () => {
      const token = localStorage.getItem('token');
      if (!token) {
        setIsAuthenticated(false);
        navigate('/login');
        return;
      }

      const isValid = await checkAuth();
      if (isValid) {
        setIsAuthenticated(true);
      } else {
        setIsAuthenticated(false);
        localStorage.removeItem('token'); // Clear invalid token
        navigate('/login');
      }
    };
    
    performAuthCheck();
  }, [navigate]);

  return isAuthenticated;
};

export default useAuth;
