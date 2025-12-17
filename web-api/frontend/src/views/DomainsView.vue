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
          />
          <small style="color: #666;">Use regular expressions to filter domains</small>
        </div>

        <div class="form-group">
          <label>Date From</label>
          <input
            v-model="filters.dateFrom"
            type="datetime-local"
          />
        </div>

        <div class="form-group">
          <label>Date To</label>
          <input
            v-model="filters.dateTo"
            type="datetime-local"
          />
        </div>

        <div class="form-group">
          <label>Limit</label>
          <input
            v-model.number="filters.limit"
            type="number"
            min="1"
            max="1000"
          />
        </div>
      </div>

      <div style="display: flex; gap: 1rem; margin-bottom: 1.5rem;">
        <button @click="applyFilters" class="btn btn-primary">Apply Filters</button>
        <button @click="resetFilters" class="btn btn-secondary">Reset</button>
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
            <tr v-for="domain in domains" :key="domain.id">
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
            <tr v-if="expandedDomain && domainDetails[expandedDomain]" class="details-row">
              <td colspan="6">
                <div class="ip-details">
                  <h3>Resolved IP Addresses</h3>
                  <div v-if="loadingDetails" class="loading">Loading IP addresses...</div>
                  <div v-else-if="domainDetails[expandedDomain].ips.length === 0" class="empty">
                    No IP addresses resolved yet
                  </div>
                  <div v-else class="ip-grid">
                    <div
                      v-for="ip in domainDetails[expandedDomain].ips"
                      :key="ip.id"
                      class="ip-card"
                      :class="'ip-' + ip.type"
                    >
                      <div class="ip-address">
                        <code>{{ ip.ip }}</code>
                      </div>
                      <div class="ip-meta">
                        <span class="ip-type">{{ ip.type.toUpperCase() }}</span>
                        <span class="ip-time">{{ formatDate(ip.time) }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </td>
            </tr>
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
import { getDomains, getDomainById } from '../api/api'
import { format } from 'date-fns'

export default {
  name: 'DomainsView',
  setup() {
    const domains = ref([])
    const loading = ref(false)
    const error = ref(null)
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
        domains.value = response.data.data
        pagination.value = {
          total: response.data.total,
          limit: response.data.limit,
          offset: response.data.offset,
          totalPages: response.data.total_pages
        }
      } catch (err) {
        error.value = err.response?.data?.error || err.message
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
          domainDetails[domainId] = response.data
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

    onMounted(() => {
      loadDomains()
    })

    return {
      domains,
      loading,
      error,
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
      formatDate
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
  padding: 1.5rem;
}

.ip-details h3 {
  margin-bottom: 1rem;
  color: #2c3e50;
}

.ip-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 1rem;
}

.ip-card {
  background: white;
  border: 1px solid #dee2e6;
  border-radius: 6px;
  padding: 1rem;
}

.ip-ipv4 {
  border-left: 4px solid #3498db;
}

.ip-ipv6 {
  border-left: 4px solid #9b59b6;
}

.ip-address {
  margin-bottom: 0.5rem;
}

.ip-address code {
  font-size: 1rem;
  font-weight: 500;
}

.ip-meta {
  display: flex;
  justify-content: space-between;
  font-size: 0.8rem;
  color: #666;
}

.ip-type {
  font-weight: 600;
  text-transform: uppercase;
}
</style>
