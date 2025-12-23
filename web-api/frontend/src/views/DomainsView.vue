<template>
  <div class="domains-view">
    <div class="card">
      <h2>Domains and Resolved IPs</h2>

      <div class="filters">
        <div class="form-group">
          <label>Domain Regex Pattern</label>
          <input
            v-model="filters.domainRegex"
            type="text"
            placeholder="^google\\..*|.*\\.com$"
            @keyup.enter="applyFilters"
          />
          <small style="color: #666;">Use regular expressions to filter domains</small>
        </div>

        <div class="form-group">
          <label>Date From</label>
          <input
            v-model="filters.dateFrom"
            type="datetime-local"
            @keyup.enter="applyFilters"
          />
        </div>

        <div class="form-group">
          <label>Date To</label>
          <input
            v-model="filters.dateTo"
            type="datetime-local"
            @keyup.enter="applyFilters"
          />
        </div>

        <div class="form-group">
          <label>Limit</label>
          <input
            v-model.number="filters.limit"
            type="number"
            min="1"
            max="1000"
            @keyup.enter="applyFilters"
          />
        </div>
      </div>

      <div style="display: flex; gap: 1rem; margin-bottom: 1.5rem;">
        <button @click="applyFilters" class="btn btn-primary">Apply Filters</button>
        <button @click="resetFilters" class="btn btn-secondary">Reset</button>
        <button
          @click="exportToExcel"
          :disabled="exporting || loading || domains.length === 0"
          class="btn btn-primary"
        >
          {{ exporting ? 'Exporting...' : 'Export to Excel' }}
        </button>
      </div>
    </div>

    <div class="card" v-if="loading">
      <div class="loading">Loading...</div>
    </div>

    <div class="card" v-else-if="error">
      <div class="error">{{ error }}</div>
    </div>

    <div class="card" v-else-if="domains.length === 0">
      <div class="empty">No domains found</div>
    </div>

    <div class="card" v-else>
      <div style="margin-bottom: 1rem; color: #666;">
        Total domains: {{ pagination.total }} | Page {{ currentPage }} of {{ pagination.totalPages }}
      </div>

      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th @click="sortBy('id')">
                ID {{ sortIcon('id') }}
              </th>
              <th @click="sortBy('domain')">
                Domain {{ sortIcon('domain') }}
              </th>
              <th @click="sortBy('time_insert')">
                First Seen {{ sortIcon('time_insert') }}
              </th>
              <th @click="sortBy('resolv_count')">
                Resolv Count {{ sortIcon('resolv_count') }}
              </th>
              <th @click="sortBy('last_resolv_time')">
                Last Resolved {{ sortIcon('last_resolv_time') }}
              </th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="domain in domains" :key="domain.id">
              <tr>
                <td>{{ domain.id }}</td>
                <td><strong>{{ domain.domain }}</strong></td>
                <td>{{ formatDate(domain.time_insert) }}</td>
                <td>
                  <span :class="{'text-success': domain.resolv_count >= domain.max_resolv}">
                    {{ domain.resolv_count }} / {{ domain.max_resolv }}
                  </span>
                </td>
                <td>{{ formatDate(domain.last_resolv_time) }}</td>
                <td>
                  <button
                    @click="toggleDetails(domain.id)"
                    class="btn btn-sm btn-primary"
                  >
                    {{ expandedDomain === domain.id ? 'Hide IPs' : 'Show IPs' }}
                  </button>
                </td>
              </tr>
              <tr v-if="expandedDomain === domain.id && domainDetails[domain.id]" class="details-row">
                <td colspan="6">
                  <div class="ip-details">
                    <h3>Resolved IP Addresses for {{ domain.domain }}</h3>
                    <div v-if="loadingDetails" class="loading">Loading IP addresses...</div>
                    <div v-else-if="!domainDetails[domain.id].ips || domainDetails[domain.id].ips.length === 0" class="empty">
                      No IP addresses resolved yet
                    </div>
                    <table v-else class="ip-table">
                      <thead>
                        <tr>
                          <th>IP Address</th>
                          <th>Type</th>
                          <th>Resolved At</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="ip in domainDetails[domain.id].ips" :key="ip.id" :class="'ip-' + ip.type">
                          <td><code>{{ ip.ip }}</code></td>
                          <td><span class="ip-type-badge" :class="'badge-' + ip.type">{{ ip.type.toUpperCase() }}</span></td>
                          <td>{{ formatDate(ip.time) }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <div class="pagination">
        <button
          @click="prevPage"
          :disabled="currentPage === 1"
          class="btn btn-secondary"
        >
          Previous
        </button>
        <span>Page {{ currentPage }} of {{ pagination.totalPages }}</span>
        <button
          @click="nextPage"
          :disabled="currentPage >= pagination.totalPages"
          class="btn btn-secondary"
        >
          Next
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, reactive, onMounted } from 'vue'
import { getDomains, getDomainById, exportDomains } from '../api/api'
import { format } from 'date-fns'

export default {
  name: 'DomainsView',
  setup() {
    const domains = ref([])
    const loading = ref(false)
    const error = ref(null)
    const exporting = ref(false)
    const currentPage = ref(1)
    const expandedDomain = ref(null)
    const loadingDetails = ref(false)
    const domainDetails = reactive({})

    const pagination = ref({
      total: 0,
      limit: 100,
      offset: 0,
      totalPages: 0
    })

    const filters = ref({
      domainRegex: '',
      dateFrom: '',
      dateTo: '',
      sortBy: 'time_insert',
      sortOrder: 'desc',
      limit: 100
    })

    const loadDomains = async () => {
      loading.value = true
      error.value = null

      try {
        const params = {
          sort_by: filters.value.sortBy,
          sort_order: filters.value.sortOrder,
          limit: filters.value.limit,
          offset: (currentPage.value - 1) * filters.value.limit
        }

        if (filters.value.domainRegex) {
          params.domain_regex = filters.value.domainRegex
        }
        if (filters.value.dateFrom) {
          params.date_from = new Date(filters.value.dateFrom).toISOString()
        }
        if (filters.value.dateTo) {
          params.date_to = new Date(filters.value.dateTo).toISOString()
        }

        const response = await getDomains(params)
        domains.value = response.data.data || []
        pagination.value = {
          total: response.data.total || 0,
          limit: response.data.limit || filters.value.limit,
          offset: response.data.offset || 0,
          totalPages: response.data.total_pages || 0
        }
      } catch (err) {
        console.error('Failed to load domains:', err)
        if (err.response?.data?.error) {
          // Server returned an error message
          error.value = `Ошибка сервера: ${err.response.data.error}`
        } else if (err.response?.status === 404) {
          error.value = 'API endpoint не найден. Проверьте конфигурацию сервера.'
        } else if (err.response?.status >= 500) {
          error.value = 'Ошибка сервера. Попробуйте позже или обратитесь к администратору.'
        } else if (err.message.includes('Network Error')) {
          error.value = 'Ошибка сети. Проверьте подключение к серверу.'
        } else {
          error.value = `Не удалось загрузить домены: ${err.message}`
        }
      } finally {
        loading.value = false
      }
    }

    const toggleDetails = async (domainId) => {
      if (expandedDomain.value === domainId) {
        expandedDomain.value = null
        return
      }

      expandedDomain.value = domainId

      if (!domainDetails[domainId]) {
        loadingDetails.value = true
        try {
          const response = await getDomainById(domainId)
          domainDetails[domainId] = {
            ...response.data,
            ips: response.data.ips || []
          }
        } catch (err) {
          error.value = err.response?.data?.error || err.message
        } finally {
          loadingDetails.value = false
        }
      }
    }

    const sortBy = (field) => {
      if (filters.value.sortBy === field) {
        filters.value.sortOrder = filters.value.sortOrder === 'asc' ? 'desc' : 'asc'
      } else {
        filters.value.sortBy = field
        filters.value.sortOrder = 'asc'
      }
      loadDomains()
    }

    const sortIcon = (field) => {
      if (filters.value.sortBy !== field) return ''
      return filters.value.sortOrder === 'asc' ? '↑' : '↓'
    }

    const applyFilters = () => {
      currentPage.value = 1
      loadDomains()
    }

    const resetFilters = () => {
      filters.value = {
        domainRegex: '',
        dateFrom: '',
        dateTo: '',
        sortBy: 'time_insert',
        sortOrder: 'desc',
        limit: 100
      }
      currentPage.value = 1
      loadDomains()
    }

    const prevPage = () => {
      if (currentPage.value > 1) {
        currentPage.value--
        loadDomains()
      }
    }

    const nextPage = () => {
      if (currentPage.value < pagination.value.totalPages) {
        currentPage.value++
        loadDomains()
      }
    }

    const formatDate = (dateStr) => {
      return format(new Date(dateStr), 'yyyy-MM-dd HH:mm:ss')
    }

    const exportToExcel = async () => {
      exporting.value = true
      error.value = null

      try {
        const params = {
          sort_by: filters.value.sortBy,
          sort_order: filters.value.sortOrder
        }

        if (filters.value.domainRegex) {
          params.domain_regex = filters.value.domainRegex
        }
        if (filters.value.dateFrom) {
          params.date_from = new Date(filters.value.dateFrom).toISOString()
        }
        if (filters.value.dateTo) {
          params.date_to = new Date(filters.value.dateTo).toISOString()
        }

        const response = await exportDomains(params)

        // Create blob and download
        const url = window.URL.createObjectURL(new Blob([response.data]))
        const link = document.createElement('a')
        link.href = url

        // Extract filename from header
        const disposition = response.headers['content-disposition']
        const filename = disposition?.match(/filename="(.+)"/)?.[1] || 'dns-domains.xlsx'

        link.download = filename
        document.body.appendChild(link)
        link.click()
        link.remove()
        window.URL.revokeObjectURL(url)
      } catch (err) {
        console.error('Failed to export domains:', err)
        if (err.response?.status === 413) {
          error.value = 'Данные слишком большие для экспорта (максимум 100,000 записей). Попробуйте уточнить фильтры.'
        } else if (err.response?.data?.error) {
          error.value = `Ошибка экспорта: ${err.response.data.error}`
        } else {
          error.value = `Не удалось экспортировать данные: ${err.message}`
        }
      } finally {
        exporting.value = false
      }
    }

    onMounted(() => {
      loadDomains()
    })

    return {
      domains,
      loading,
      error,
      exporting,
      filters,
      currentPage,
      pagination,
      expandedDomain,
      loadingDetails,
      domainDetails,
      toggleDetails,
      sortBy,
      sortIcon,
      applyFilters,
      resetFilters,
      prevPage,
      nextPage,
      formatDate,
      exportToExcel
    }
  }
}
</script>

<style scoped>
code {
  background: #f4f4f4;
  padding: 0.2rem 0.4rem;
  border-radius: 3px;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
}

.text-success {
  color: #2ecc71;
  font-weight: 600;
}

.btn-sm {
  padding: 0.3rem 0.8rem;
  font-size: 0.85rem;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.details-row {
  background: #f8f9fa !important;
}

.details-row:hover {
  background: #f8f9fa !important;
}

.ip-details {
  padding: 1rem 1.5rem;
}

.ip-details h3 {
  margin-bottom: 1rem;
  color: #2c3e50;
  font-size: 1rem;
}

.ip-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
  margin-top: 0.5rem;
}

.ip-table thead {
  background: #e9ecef;
}

.ip-table th {
  padding: 0.5rem;
  text-align: left;
  font-weight: 600;
  color: #495057;
  border-bottom: 2px solid #dee2e6;
  font-size: 0.8rem;
}

.ip-table td {
  padding: 0.5rem;
  border-bottom: 1px solid #e9ecef;
}

.ip-table tbody tr:hover {
  background: #f8f9fa;
}

.ip-table .ip-ipv4 {
  border-left: 3px solid #3498db;
}

.ip-table .ip-ipv6 {
  border-left: 3px solid #9b59b6;
}

.ip-table code {
  background: #f4f4f4;
  padding: 0.15rem 0.4rem;
  border-radius: 3px;
  font-family: 'Courier New', monospace;
  font-size: 0.75rem;
}

.ip-type-badge {
  display: inline-block;
  padding: 0.15rem 0.5rem;
  border-radius: 10px;
  font-size: 0.7rem;
  font-weight: 600;
}

.ip-type-badge.badge-ipv4 {
  background: #3498db;
  color: white;
}

.ip-type-badge.badge-ipv6 {
  background: #9b59b6;
  color: white;
}
</style>
