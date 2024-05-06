const BASE_URL = 'http://localhost:3001'
export async function startPump() {
  const response = await fetch(BASE_URL + '/api/start');
  if (!response.ok) {
    
  }
}