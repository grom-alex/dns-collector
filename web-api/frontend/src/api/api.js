import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json'
  }
})

export const getStats = (params) => {
  return api.get('/stats', { params })
}

export const getDomains = (params) => {
  return api.get('/domains', { params })
}

export const getDomainById = (id) => {
  return api.get(`/domains/${id}`)
}

export const exportStats = (params) => {
  return api.get('/stats/export', { params, responseType: 'blob' })
}

export const exportDomains = (params) => {
  return api.get('/domains/export', { params, responseType: 'blob' })
}

export default api
