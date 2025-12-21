<template>
  <div class="stats-view">
    <div class="card">
      <h2>DNS Query Statistics</h2>

      <div class="filters">
        <div class="form-group">
          <label>Client IPs (comma-separated)</label>
          <input
            v-model="filters.clientIPs"
            type="text"
            placeholder="192.168.1.1, 192.168.1.2"
            @keyup.enter="applyFilters"
          />
        </div>

        <div class="form-group">
          <label>Subnet (CIDR)</label>
          <input
            v-model="filters.subnet"
            type="text"
            placeholder="192.168.1.0/24"
            @keyup.enter="applyFilters"
          />
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
      </div>
    </div>

    <div class="card" v-if="loading">
      <div class="loading">Loading...</div>
    </div>

    <div class="card" v-else-if="error">
      <div class="error">{{ error }}</div>
    </div>

    <div class="card" v-else-if="stats.length === 0">
      <div class="empty">No statistics found</div>
    </div>

    <div class="card" v-else>
      <div style="margin-bottom: 1rem; color: #666;">
        Total records: {{ pagination.total }} | Page {{ currentPage }} of {{ pagination.totalPages }}
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
              <th @click="sortBy('client_ip')">
                Client IP {{ sortIcon('client_ip') }}
              </th>
              <th @click="sortBy('rtype')">
                Type {{ sortIcon('rtype') }}
              </th>
              <th @click="sortBy('timestamp')">
                Timestamp {{ sortIcon('timestamp') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="stat in stats" :key="stat.id">
              <td>{{ stat.id }}</td>
              <td><strong>{{ stat.domain }}</strong></td>
              <td><code>{{ stat.client_ip }}</code></td>
              <td>
                <span class="badge" :class="'badge-' + stat.rtype">
                  {{ stat.rtype }}
                </span>
              </td>
              <td>{{ formatDate(stat.timestamp) }}</td>
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
import { ref, onMounted } from 'vue'
import { getStats } from '../api/api'
import { format } from 'date-fns'

export default {
  name: 'StatsView',
  setup() {
    const stats = ref([])
    const loading = ref(false)
    const error = ref(null)
    const currentPage = ref(1)
    const pagination = ref({
      total: 0,
      limit: 100,
      offset: 0,
      totalPages: 0
    })

    const filters = ref({
      clientIPs: '',
      subnet: '',
      dateFrom: '',
      dateTo: '',
      sortBy: 'timestamp',
      sortOrder: 'desc',
      limit: 100
    })

    const loadStats = async () => {
      loading.value = true
      error.value = null

      try {
        const params = {
          sort_by: filters.value.sortBy,
          sort_order: filters.value.sortOrder,
          limit: filters.value.limit,
          offset: (currentPage.value - 1) * filters.value.limit
        }

        if (filters.value.clientIPs) {
          params.client_ips = filters.value.clientIPs
        }
        if (filters.value.subnet) {
          params.subnet = filters.value.subnet
        }
        if (filters.value.dateFrom) {
          params.date_from = new Date(filters.value.dateFrom).toISOString()
        }
        if (filters.value.dateTo) {
          params.date_to = new Date(filters.value.dateTo).toISOString()
        }

        const response = await getStats(params)
        stats.value = response.data.data || []
        pagination.value = {
          total: response.data.total || 0,
          limit: response.data.limit || filters.value.limit,
          offset: response.data.offset || 0,
          totalPages: response.data.total_pages || 0
        }
      } catch (err) {
        console.error('Failed to load stats:', err)
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
          error.value = `Не удалось загрузить статистику: ${err.message}`
        }
      } finally {
        loading.value = false
      }
    }

    const sortBy = (field) => {
      if (filters.value.sortBy === field) {
        filters.value.sortOrder = filters.value.sortOrder === 'asc' ? 'desc' : 'asc'
      } else {
        filters.value.sortBy = field
        filters.value.sortOrder = 'asc'
      }
      loadStats()
    }

    const sortIcon = (field) => {
      if (filters.value.sortBy !== field) return ''
      return filters.value.sortOrder === 'asc' ? '↑' : '↓'
    }

    const applyFilters = () => {
      currentPage.value = 1
      loadStats()
    }

    const resetFilters = () => {
      filters.value = {
        clientIPs: '',
        subnet: '',
        dateFrom: '',
        dateTo: '',
        sortBy: 'timestamp',
        sortOrder: 'desc',
        limit: 100
      }
      currentPage.value = 1
      loadStats()
    }

    const prevPage = () => {
      if (currentPage.value > 1) {
        currentPage.value--
        loadStats()
      }
    }

    const nextPage = () => {
      if (currentPage.value < pagination.value.totalPages) {
        currentPage.value++
        loadStats()
      }
    }

    const formatDate = (dateStr) => {
      return format(new Date(dateStr), 'yyyy-MM-dd HH:mm:ss')
    }

    onMounted(() => {
      loadStats()
    })

    return {
      stats,
      loading,
      error,
      filters,
      currentPage,
      pagination,
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

.badge {
  display: inline-block;
  padding: 0.25rem 0.6rem;
  border-radius: 12px;
  font-size: 0.8rem;
  font-weight: 500;
}

.badge-dns {
  background: #3498db;
  color: white;
}

.badge-cache {
  background: #2ecc71;
  color: white;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
