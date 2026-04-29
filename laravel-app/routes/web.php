<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\WebSocketTicketController;

/*
|--------------------------------------------------------------------------
| Web Routes
|--------------------------------------------------------------------------
|
| Here is where you can register web routes for your application. These
| routes are loaded by the RouteServiceProvider within a group which
| contains the "web" middleware group (sessions, CSRF etc). Now create something great!
|
*/

// Client endpoint to request a short-lived ticket using Session / Cookie auth.
Route::middleware('auth')->post('/ws/generate-ticket', [WebSocketTicketController::class, 'generateTicket']);