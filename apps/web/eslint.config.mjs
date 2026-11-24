import nextConfig from "@silieco/eslint-config/next";
export default [...nextConfig, { ignores: [".next/", ".source/"] }];
