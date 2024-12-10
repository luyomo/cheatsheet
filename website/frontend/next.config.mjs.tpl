/** @type {import('next').NextConfig} */
const nextConfig = {
    reactStrictMode: true,
    basePath: '/example',
    env: {
        GAPI_CLIENT_ID: "ddddddddddddd-dddddddddddddddddddddddddddddddd.apps.googleusercontent.com",
        AZURE_CLIENT_ID: "dddddddddddd-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.apps.googleusercontent.com",
        REDIRECT_URI: "https://www.51yomo.net/prefix/dashboard",
    },
};

export default nextConfig;
