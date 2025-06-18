import axios, { AxiosError } from 'axios';
import { Device } from '../models/device.model';
import { SystemStatus } from '../models/systemStatus.model';
import { EventLog } from '../models/eventLog.model';
import { Network } from '../models/network.model';
import { API_BASE_URL } from '../config';

// Logger function
const logger = {
  info: (message: string, ...args: any[]) => {
    if (process.env.REACT_APP_LOG_LEVEL === 'info' || process.env.REACT_APP_LOG_LEVEL === 'debug') {
      console.info(`[INFO] ${message}`, ...args);
    }
  },
  error: (message: string, ...args: any[]) => {
    console.error(`[ERROR] ${message}`, ...args);
  },
  debug: (message: string, ...args: any[]) => {
    if (process.env.REACT_APP_LOG_LEVEL === 'debug') {
      console.debug(`[DEBUG] ${message}`, ...args);
    }
  }
};

// Create axios instance with configuration from environment variables
const axiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: parseInt(process.env.REACT_APP_API_TIMEOUT || '30000', 10),
  headers: {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  },
  withCredentials: false, // Disable credentials for CORS
});

// Request interceptor - adds auth token and logs requests
axiosInstance.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers['Authorization'] = `Bearer ${token}`;
    }
    
    logger.debug(`API Request: ${config.method?.toUpperCase()} ${config.url}`, {
      params: config.params,
      data: config.data
    });
    
    return config;
  },
  (error) => {
    logger.error('API Request Error:', error);
    return Promise.reject(error);
  }
);

// Response interceptor - logs responses and errors
axiosInstance.interceptors.response.use(
  (response) => {
    logger.debug(`API Response: ${response.status} ${response.config.method?.toUpperCase()} ${response.config.url}`);
    return response;
  },
  (error: AxiosError) => {
    logger.error('API Response Error:', {
      status: error.response?.status,
      url: error.config?.url,
      method: error.config?.method?.toUpperCase(),
      data: error.response?.data
    });
    
    // Handle token expiration (401) errors
    if (error.response?.status === 401) {
      // Clear token and redirect to login
      localStorage.removeItem('token');
      window.location.href = '/login';
    }
    
    return Promise.reject(error);
  }
);

// API endpoints
export const fetchDevices = async (): Promise<Device[]> => {
  try {
    const response = await axiosInstance.get<Device[]>('/devices');
    return response.data;
  } catch (error) {
    logger.error("Error fetching devices:", error);
    throw error;
  }
};

export const fetchSystemStatus = async (): Promise<SystemStatus> => {
  try {
    const response = await axiosInstance.get<SystemStatus>('/system-status/latest');
    return response.data;
  } catch (error) {
    logger.error("Error fetching system-status:", error);
    throw error;
  }
};

export const fetchEventLogs = async (): Promise<EventLog[]> => {
  try {
    const response = await axiosInstance.get<EventLog[]>('/event-log');
    return response.data;
  } catch (error) {
    logger.error("Error fetching event logs:", error);
    throw error;
  }
};

export const fetchNetwork = async (): Promise<Network> => {
  try {
    const response = await axiosInstance.get<Network>(`/network`);
    return response.data;
  } catch (error) {
    logger.error("Error fetching network:", error);
    throw error;
  }
};

// Login function
export interface LoginCredentials {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
}

export const login = async (credentials: LoginCredentials): Promise<LoginResponse> => {
  try {
    const response = await axiosInstance.post<LoginResponse>('/login', credentials);
    return response.data;
  } catch (error) {
    logger.error("Login error:", error);
    throw error;
  }
};

// Check auth function
export const checkAuth = async (): Promise<boolean> => {
  try {
    const response = await axiosInstance.get('/check-auth');
    return response.status === 200;
  } catch (error) {
    logger.error("Auth check error:", error);
    return false;
  }
};

// Export logger to be used in other files
export { logger };

export default axiosInstance;
