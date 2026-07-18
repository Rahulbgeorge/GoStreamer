import { execSync } from 'child_process';

console.log('Loading Node.js Production Build Config...');

// Set the production API base URL env variable
process.env.VITE_API_BASE = 'https://web-dustin.offlinesys.shop/api/v1';
// process.env.VITE_API_BASE = "http://localhost:8000/api/v1";

console.log('Building frontend production assets with URL: ' + process.env.VITE_API_BASE);

try {
  execSync('npm run build', { stdio: 'inherit', env: process.env });
  console.log('Frontend built successfully with embedded API Base URL!');
} catch (error) {
  console.error('Frontend build failed:', error);
  process.exit(1);
}
