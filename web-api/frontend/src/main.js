import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import StatsView from './views/StatsView.vue'
import DomainsView from './views/DomainsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/stats' },
    { path: '/stats', component: StatsView },
    { path: '/domains', component: DomainsView }
  ]
})

const app = createApp(App)
app.use(router)
app.mount('#app')
