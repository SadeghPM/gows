<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\WebSocketTicketController;

/*
|--------------------------------------------------------------------------
| API Routes
|--------------------------------------------------------------------------
*/

// 1. Internal endpoint designated for Go server to validate a ticket.
// Protected statically via WS_INTERNAL_SECRET env var inside controller.
// DO NOT protect this with normal auth middleware like sanctum.
Route::post('/internal/ws/validate-ticket', [WebSocketTicketController::class, 'validateTicket']);

