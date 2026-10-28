// The browser's pages: the front page and the login form come with the first script, every other page on demand.
import { LandingPage } from './LandingPage';
import LoginRoute from './LoginPage';
import { NotFoundPage } from './NotFoundPage';
import { lazyRoute, provideRoute } from './pages';

provideRoute('home', LandingPage);
provideRoute('notFound', NotFoundPage);
provideRoute('login', LoginRoute);
lazyRoute('privacy', () => import('./PrivacyPage'));
lazyRoute('register', () => import('./RegisterPage'));
lazyRoute('lobby', () => import('./LobbyPage'));
lazyRoute('admin', () => import('./admin/AdminPage'));
lazyRoute('player', () => import('./ProfilePage'));
lazyRoute('game', () => import('./GamePage'));
lazyRoute('join', () => import('./JoinPage'));
lazyRoute('worlds', () => import('./worlds/WorldsPage'));
lazyRoute('world', () => import('./worlds/WorldPage'));
lazyRoute('units', () => import('./encyclopedia/UnitsPage'));
lazyRoute('unit', () => import('./encyclopedia/UnitPage'));
lazyRoute('buildings', () => import('./encyclopedia/BuildingsPage'));
lazyRoute('building', () => import('./encyclopedia/BuildingPage'));
lazyRoute('howToPlay', () => import('./content/HowToPlayPage'));
lazyRoute('guide', () => import('./content/GuidePage'));
lazyRoute('faq', () => import('./content/FaqPage'));
lazyRoute('changelog', () => import('./content/ChangelogPage'));
lazyRoute('terms', () => import('./content/TermsPage'));
lazyRoute('report', () => import('./ReportPage'));
