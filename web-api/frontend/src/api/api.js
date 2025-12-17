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

export default api
