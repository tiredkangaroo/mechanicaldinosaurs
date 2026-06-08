<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { user } from '$lib/stores/user';

	let { children } = $props();
	let username = $state('');
	let password = $state('');
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
{#if $user}
	{@render children()}
{:else}
	<div class="flex h-full w-full flex-col items-center justify-center bg-orange-100">
		<div
			class="flex flex-col items-center gap-4 rounded-xl border-4 border-orange-800 bg-white px-48 py-8 shadow-md"
		>
			<h1 class="text-4xl font-bold">login</h1>
			<select
				class="w-full rounded-xl border-2 border-orange-800 px-24 py-2 text-center"
				bind:value={username}
			>
				<option value="" disabled selected>Select your name</option>
				<option value="ajitesh">ajitesh</option>
				<option value="john">john</option>
				<option value="doe">doe</option>
				<option value="admin">admin</option>
				<option value="editor">editor</option>
				<option value="viewer">viewer</option>
			</select>
			<input
				type="password"
				placeholder="password"
				class="w-full rounded-xl border-2 border-orange-800 px-24 py-2 text-center"
				bind:value={password}
			/>
			<button
				class="w-full rounded-xl bg-orange-800 px-24 py-2 text-center text-orange-100 transition-colors duration-500 disabled:bg-orange-200"
				disabled={!username || !password}>login</button
			>
		</div>
	</div>
{/if}
