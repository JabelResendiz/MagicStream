import axios from 'axios'
const apiURL = import.meta.env.VITE_API_BASE_URL

export default axios.create({
    baseURL: apiURL,
    headers: { 'Content-Type': 'application/json' }
})