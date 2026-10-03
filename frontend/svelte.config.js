import adapter from '@sveltejs/adapter-static';
export default { kit: { adapter: adapter({pages:'../web/build',assets:'../web/build',fallback:'index.html'}) } };
